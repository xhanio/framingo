package planner

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
)

func (m *manager) Add(plan *entity.Plan) error {
	if plan.ID == "" {
		plan.ID = uuid.NewString()
	}
	if plan.Kind != "" {
		if plan.Task == nil {
			return errors.InvalidArgument.Newf("plan %s has no task", plan.ID)
		}
		if plan.Kind != entity.PlanKindCommand {
			return errors.InvalidArgument.Newf("plan %s has an unknown kind %q", plan.ID, plan.Kind)
		}
		// the planner passes each firing's run ID and source in Task.Params
		if plan.Task.Params != nil {
			return errors.InvalidArgument.Newf("plan %s: a %s plan takes its params in Plan.Params, and leaves Task.Params empty", plan.ID, plan.Kind)
		}
		fn, err := m.commandJob(plan)
		if err != nil {
			return err
		}
		plan.Task.Job = job.New(plan.ID, fn, job.WithLabels(plan.Metadata), job.WithLogger(m.log))
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
	m.plans[plan.ID] = plan
	return nil
}

func (m *manager) Trigger(id string, source entity.PlanRunSource) (string, error) {
	if source == "" {
		return "", errors.InvalidArgument.Newf("failed to trigger plan %s: no source", id)
	}
	m.RLock()
	defer m.RUnlock()
	plan, ok := m.plans[id]
	if !ok {
		return "", errors.NotFound.Newf("failed to trigger plan %s: plan not found", id)
	}
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

func (m *manager) Cancel(id string) error {
	m.RLock()
	defer m.RUnlock()
	plan, ok := m.plans[id]
	if !ok {
		return errors.NotFound.Newf("failed to cancel plan %s: plan not found", id)
	}
	err := m.tm.Cancel(plan.Task.ID())
	if errors.Is(err, errors.NotFound) {
		// the plan is here, its one-shot task finished and left the task manager
		return errors.Conflict.Newf("plan %s is not running", id)
	}
	return errors.Wrap(err)
}

func (m *manager) Delete(id string, force bool) (*entity.PlanRun, error) {
	m.Lock()
	plan, ok := m.plans[id]
	if !ok {
		m.Unlock()
		return nil, errors.NotFound.Newf("plan %s not found", id)
	}
	delete(m.plans, id)
	m.Unlock()
	if force {
		// an ended context cancels and then kills at once; the wait below lets the kill land
		ended, end := context.WithCancel(context.Background())
		end()
		_ = m.tm.Remove(ended, plan.Task)
	}
	limit := endLimit(plan, force)
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	if err := m.tm.Remove(ctx, plan.Task); err != nil && limit > 0 {
		m.log.Warnf("plan %s is still running %s after it was deleted", id, limit)
	}
	return lastRun(plan), nil
}

// A command is killed once its grace period is up, so waiting longer only helps one stuck in the
// kernel. Go cannot kill a goroutine, so a forced plan with a job of its own is not waited for.
func endLimit(plan *entity.Plan, force bool) time.Duration {
	switch {
	case plan.Kind == "" && force:
		return 0
	case plan.Kind == "":
		return plan.Task.Grace()
	case force:
		return commandKillWait
	default:
		return plan.Task.Grace() + commandKillWait
	}
}

func (m *manager) GetResult(id string) (any, error) {
	m.RLock()
	defer m.RUnlock()
	if plan, ok := m.plans[id]; ok {
		return plan.Task.Job.Result(), nil
	}
	return nil, errors.NotFound.Newf("failed to get result of plan %s: plan not found", id)
}

func (m *manager) LastRun(id string) (*entity.PlanRun, error) {
	m.RLock()
	defer m.RUnlock()
	plan, ok := m.plans[id]
	if !ok {
		return nil, errors.NotFound.Newf("failed to get the last run of plan %s: plan not found", id)
	}
	return lastRun(plan), nil
}

func lastRun(plan *entity.Plan) *entity.PlanRun {
	if plan.Kind == "" {
		return nil
	}
	run, ok := plan.Task.Job.Result().(entity.PlanRun)
	if !ok {
		return nil
	}
	return &run
}

// By path, not run ID, since the planner only remembers the latest run. The check is lexical:
// the planner made these files.
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

func (m *manager) stats(all bool) []*entity.PlannerStats {
	result := make([]*entity.PlannerStats, 0)
	for _, plan := range m.plans {
		t := plan.Task.Job
		if !all && t.State() == job.StateSucceeded {
			continue
		}
		ts := t.Stats()
		stats := &entity.PlannerStats{
			ID:            ts.ID,
			State:         ts.State,
			Progress:      ts.Progress,
			StartedAt:     ts.StartedAt,
			ExecutionTime: ts.ExecutionTime,
			Labels:        ts.Labels,
			Error:         ts.Error,
		}
		stats.Schedule = plan.Task.Schedule
		ss := m.tm.Stats(t.ID())
		if ss != nil {
			stats.Cooldown = ss.Cooldown
			stats.Retries = ss.Retries
		}
		result = append(result, stats)
	}
	sort.Slice(result, func(i, t int) bool {
		return result[i].StartedAt.Before(result[t].StartedAt)
	})
	return result
}

func (m *manager) Stats(opts entity.PlannerStatsOptions) ([]*entity.PlannerStats, error) {
	selector, err := labels.Parse(opts.Selector)
	if err != nil {
		return nil, errors.InvalidArgument.Wrap(err)
	}
	m.RLock()
	defer m.RUnlock()
	var result []*entity.PlannerStats
	for _, plan := range m.stats(opts.All) {
		if selector.Empty() || selector.Matches(plan.Labels) {
			result = append(result, plan)
		}
	}
	return result, nil
}
