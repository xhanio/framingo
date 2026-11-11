package plan

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/xhanio/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/plans"
)

// Add keeps a copy of the plan and its task: the caller's stay as they were.
func (m *manager) Add(plan *entity.Plan) error {
	if plan.ID == "" {
		return errors.InvalidArgument.Newf("a plan needs an ID")
	}
	if plan.Task == nil {
		return errors.InvalidArgument.Newf("plan %s has no task", plan.ID)
	}
	if plan.Task.IsOneShot() {
		return errors.InvalidArgument.Newf("plan %s needs a schedule or Manual: a plan stays until Delete", plan.ID)
	}
	p, task := *plan, *plan.Task
	p.Task = &task
	reg := &registration{plan: &p}
	switch {
	case p.Kind == "":
		return errors.InvalidArgument.Newf("plan %s has no kind", p.ID)
	case p.Kind != entity.PlanKindCommand:
		return errors.InvalidArgument.Newf("plan %s has an unknown kind %q", p.ID, p.Kind)
	case task.Job != nil:
		return errors.InvalidArgument.Newf("plan %s: the plan manager builds its task's job, from its kind", p.ID)
	case task.Params != nil:
		// Task.Params is where each firing of Trigger passes its run
		return errors.InvalidArgument.Newf("plan %s: a %s plan takes its params in Plan.Params, and leaves Task.Params empty", p.ID, p.Kind)
	}
	cmd, err := plans.NewCommand(&p)
	if err != nil {
		return err
	}
	// under the plan's ID, which the task manager knows it by
	task.Job = job.New(p.ID, m.runJob(reg, cmd.Header(), cmd.Run), job.WithLabels(p.Metadata), job.WithLogger(m.log))
	m.Lock()
	defer m.Unlock()
	if err := m.tm.Add(&task); err != nil {
		return errors.Wrap(err)
	}
	m.plans[p.ID] = reg
	return nil
}

func (m *manager) Trigger(id string, source entity.PlanRunSource) (string, error) {
	if source == "" {
		return "", errors.InvalidArgument.Newf("failed to trigger plan %s: no source", id)
	}
	// the write lock, so that a Cancel comes before or after the firing is recorded
	m.Lock()
	defer m.Unlock()
	reg, ok := m.plans[id]
	if !ok {
		return "", errors.NotFound.Newf("failed to trigger plan %s: plan not found", id)
	}
	tr := &triggered{id: uuid.NewString(), source: source, at: time.Now()}
	if err := m.tm.Trigger(reg.plan.Task.ID(), tr); err != nil {
		return "", errors.Wrap(err)
	}
	reg.setPending(tr)
	return tr.id, nil
}

func (m *manager) Cancel(id string) error {
	m.Lock()
	defer m.Unlock()
	reg, ok := m.plans[id]
	if !ok {
		return errors.NotFound.Newf("failed to cancel plan %s: plan not found", id)
	}
	if err := m.tm.Cancel(reg.plan.Task.ID()); err != nil {
		return errors.Wrap(err)
	}
	m.endUnstarted(reg)
	return nil
}

func (m *manager) Delete(ctx context.Context, id string) (*entity.PlanRun, error) {
	m.Lock()
	reg, ok := m.plans[id]
	if !ok {
		m.Unlock()
		return nil, errors.NotFound.Newf("plan %s not found", id)
	}
	delete(m.plans, id)
	m.Unlock()
	err := m.tm.Remove(ctx, reg.plan.Task)
	m.endUnstarted(reg)
	if err != nil {
		return reg.lastRun(), errors.Wrapf(err, "plan %s deleted with its run still going", id)
	}
	return reg.lastRun(), nil
}

func (m *manager) LastRun(id string) (*entity.PlanRun, error) {
	m.RLock()
	defer m.RUnlock()
	reg, ok := m.plans[id]
	if !ok {
		return nil, errors.NotFound.Newf("failed to get the last run of plan %s: plan not found", id)
	}
	return reg.lastRun(), nil
}

// By path, not run ID, since the plan manager only remembers the latest run. The check is lexical:
// the plan manager made these files.
func (m *manager) OpenRunLog(path string, tailBytes int64) (io.ReadCloser, error) {
	if m.logDir == "" {
		return nil, errors.NotFound.Newf("failed to open run log %s: no log dir is configured", path)
	}
	root, err := filepath.Abs(m.logDir)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if rel, err := filepath.Rel(root, abs); err != nil || rel == "." || !filepath.IsLocal(rel) {
		return nil, errors.Forbidden.Newf("failed to open run log %s: it is not under the log dir", path)
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NotFound.Newf("failed to open run log %s: the file is gone", path)
		}
		return nil, errors.Wrap(err)
	}
	if tailBytes > 0 {
		if st, err := f.Stat(); err == nil && st.Size() > tailBytes {
			if _, err := f.Seek(st.Size()-tailBytes, io.SeekStart); err != nil {
				f.Close()
				return nil, errors.Wrap(err)
			}
		}
	}
	return f, nil
}

// stats are the plans' stats, from their jobs and their tasks, in the order they started; a plan
// whose job succeeded is left out unless all.
func (m *manager) stats(all bool) []*entity.PlanStats {
	result := make([]*entity.PlanStats, 0)
	for _, reg := range m.plans {
		plan := reg.plan
		j := plan.Task.Job
		if !all && j.State() == job.StateSucceeded {
			continue
		}
		js := j.Stats()
		st := &entity.PlanStats{
			ID:            plan.ID,
			Schedule:      plan.Task.Schedule,
			State:         js.State,
			Progress:      js.Progress,
			StartedAt:     js.StartedAt,
			ExecutionTime: js.ExecutionTime,
			Labels:        plan.Metadata,
			Error:         js.Error,
		}
		if ts := m.tm.Stats(j.ID()); ts != nil {
			st.Cooldown = ts.Cooldown
			st.Retries = ts.Retries
		}
		result = append(result, st)
	}
	sort.Slice(result, func(a, b int) bool {
		return result[a].StartedAt.Before(result[b].StartedAt)
	})
	return result
}

func (m *manager) Stats(opts entity.PlanStatsOptions) ([]*entity.PlanStats, error) {
	selector, err := labels.Parse(opts.Selector)
	if err != nil {
		return nil, errors.InvalidArgument.Wrap(err)
	}
	m.RLock()
	defer m.RUnlock()
	var result []*entity.PlanStats
	for _, st := range m.stats(opts.All) {
		if selector.Empty() || selector.Matches(st.Labels) {
			result = append(result, st)
		}
	}
	return result, nil
}
