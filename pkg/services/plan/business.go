package plan

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/xhanio/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/plans"
)

func (m *manager) Add(plan *entity.Plan) error {
	if plan.ID == "" {
		plan.ID = uuid.NewString()
	}
	reg := &registration{plan: plan}
	if plan.Kind != "" {
		if plan.Task == nil {
			return errors.InvalidArgument.Newf("plan %s has no task", plan.ID)
		}
		if plan.Kind != entity.PlanKindCommand {
			return errors.InvalidArgument.Newf("plan %s has an unknown kind %q", plan.ID, plan.Kind)
		}
		// the plan manager passes each firing's run ID and source in Task.Params
		if plan.Task.Params != nil {
			return errors.InvalidArgument.Newf("plan %s: a %s plan takes its params in Plan.Params, and leaves Task.Params empty", plan.ID, plan.Kind)
		}
		cmd, err := plans.NewCommand(plan)
		if err != nil {
			return err
		}
		plan.Task.Job = job.New(plan.ID, m.runJob(reg, cmd.Header(), cmd.Run), job.WithLabels(plan.Metadata), job.WithLogger(m.log))
	}
	m.Lock()
	defer m.Unlock()
	if !plan.Task.IsValid() {
		return nil
	}
	err := m.tm.Add(plan.Task)
	if err != nil {
		return errors.Wrap(err)
	}
	m.plans[plan.ID] = reg
	return nil
}

func (m *manager) Trigger(id string, source entity.PlanRunSource) (string, error) {
	if source == "" {
		return "", errors.InvalidArgument.Newf("failed to trigger plan %s: no source", id)
	}
	m.RLock()
	defer m.RUnlock()
	reg, ok := m.plans[id]
	if !ok {
		return "", errors.NotFound.Newf("failed to trigger plan %s: plan not found", id)
	}
	plan := reg.plan
	var (
		runID  string
		params any
	)
	if plan.Kind != "" {
		runID = uuid.NewString()
		params = &triggeredRun{id: runID, source: source}
	}
	if err := m.tm.Trigger(plan.Task.ID(), params); err != nil {
		return "", errors.Wrap(err)
	}
	return runID, nil
}

// triggeredRun goes with a firing of Trigger: all its runs take the source, only the first
// the ID Trigger returned, as retries and requeues are runs of their own.
type triggeredRun struct {
	id      string
	source  entity.PlanRunSource
	claimed atomic.Bool
}

func (m *manager) Cancel(id string) error {
	m.RLock()
	defer m.RUnlock()
	reg, ok := m.plans[id]
	if !ok {
		return errors.NotFound.Newf("failed to cancel plan %s: plan not found", id)
	}
	err := m.tm.Cancel(reg.plan.Task.ID())
	if errors.Is(err, errors.NotFound) {
		// the plan is here, its one-shot task finished and left the task manager
		return errors.Conflict.Newf("plan %s is not running", id)
	}
	return errors.Wrap(err)
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
	if err := m.tm.Remove(ctx, reg.plan.Task); err != nil {
		return reg.lastRun(), errors.Wrapf(err, "plan %s deleted with its run still going", id)
	}
	return reg.lastRun(), nil
}

func (m *manager) Result(id string) (any, error) {
	m.RLock()
	defer m.RUnlock()
	if reg, ok := m.plans[id]; ok {
		return reg.plan.Task.Job.Result(), nil
	}
	return nil, errors.NotFound.Newf("failed to get result of plan %s: plan not found", id)
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
			ID:            js.ID,
			Schedule:      plan.Task.Schedule,
			State:         js.State,
			Progress:      js.Progress,
			StartedAt:     js.StartedAt,
			ExecutionTime: js.ExecutionTime,
			Labels:        js.Labels,
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
