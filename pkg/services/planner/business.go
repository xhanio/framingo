package planner

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/xhanio/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func (m *manager) Add(todo *entity.Plan) error {
	if todo.ID == "" {
		todo.ID = uuid.NewString()
	}
	if todo.Kind != "" {
		if err := m.buildJob(todo); err != nil {
			return err
		}
	}
	m.Lock()
	defer m.Unlock()
	if !todo.Task.IsValid() {
		return nil
	}
	err := m.tm.Add(todo.Task)
	if err != nil {
		return errors.Wrap(err)
	}
	m.todos[todo.ID] = todo
	return nil
}

func (m *manager) Trigger(id string, source entity.PlanRunSource) (string, error) {
	if source == "" {
		return "", errors.InvalidArgument.Newf("failed to trigger todo %s: no source", id)
	}
	m.RLock()
	defer m.RUnlock()
	todo, ok := m.todos[id]
	if !ok {
		return "", errors.NotFound.Newf("failed to trigger todo %s: todo id not found", id)
	}
	var (
		runID  string
		params any
	)
	if todo.Kind != "" {
		runID = uuid.NewString()
		params = &triggeredRun{id: runID, source: source}
	}
	if err := m.tm.Trigger(todo.Task.ID(), params); err != nil {
		return "", errors.Wrap(err)
	}
	return runID, nil
}

func (m *manager) Cancel(id string) error {
	m.RLock()
	defer m.RUnlock()
	todo, ok := m.todos[id]
	if !ok {
		return errors.NotFound.Newf("failed to cancel todo %s: todo id not found", id)
	}
	err := m.tm.Cancel(todo.Task.ID())
	if errors.Is(err, errors.NotFound) {
		// the plan is here, its one-shot task finished and left the task manager
		return errors.Conflict.Newf("todo %s is not running", id)
	}
	return errors.Wrap(err)
}

func (m *manager) Delete(id string, force bool) (*entity.PlanRun, error) {
	m.Lock()
	todo, ok := m.todos[id]
	if !ok {
		m.Unlock()
		return nil, errors.NotFound.Newf("todo id %s not found", id)
	}
	ending := m.forget(todo)
	m.Unlock()
	m.awaitEnd(force, ending)
	return lastRun(todo), nil
}

// ending is a forgotten plan and the channel that closes once its execution has ended.
type ending struct {
	todo  *entity.Plan
	ended <-chan struct{}
}

// forget takes the plan off the task manager, which ends what it is doing, cron entry first.
// The caller holds the lock.
func (m *manager) forget(todo *entity.Plan) ending {
	delete(m.todos, todo.ID)
	return ending{todo: todo, ended: m.tm.Remove(todo.Task)}
}

// awaitEnd waits for forgotten plans. Their limits run from the same moment, so the wait lasts
// as long as the slowest of them, not the sum. With force, each job is sent SignalKill; what
// it does with that is up to the job.
func (m *manager) awaitEnd(force bool, endings ...ending) {
	if force {
		for _, e := range endings {
			e.todo.Task.Job.Signal(job.SignalKill)
		}
	}
	begin := time.Now()
	for _, e := range endings {
		limit := endLimit(e.todo, force)
		timer := time.NewTimer(time.Until(begin.Add(limit)))
		select {
		case <-e.ended:
		case <-timer.C:
			if limit > 0 {
				m.log.Warnf("todo %s is still running %s after it was stopped", e.todo.ID, limit)
			}
		}
		timer.Stop()
	}
}

// endLimit is how long a stopped plan is waited for. A command is killed once its grace period
// is up, so only one stuck in the kernel takes longer, and waiting on is not worth stopping for.
// Go cannot kill a goroutine, so a forced plan with a job of its own is not waited for.
func endLimit(todo *entity.Plan, force bool) time.Duration {
	switch {
	case todo.Kind == "" && force:
		return 0
	case todo.Kind == "":
		return todo.Task.Grace()
	case force:
		return commandKillWait
	default:
		return todo.Task.Grace() + commandKillWait
	}
}

func (m *manager) GetResult(id string) (any, error) {
	m.RLock()
	defer m.RUnlock()
	if todo, ok := m.todos[id]; ok {
		return todo.Task.Job.Result(), nil
	}
	return nil, errors.NotFound.Newf("failed to get result of todo %s: todo id not found", id)
}

func (m *manager) LastRun(id string) (*entity.PlanRun, error) {
	m.RLock()
	defer m.RUnlock()
	todo, ok := m.todos[id]
	if !ok {
		return nil, errors.NotFound.Newf("failed to get the last run of todo %s: todo id not found", id)
	}
	return lastRun(todo), nil
}

func lastRun(todo *entity.Plan) *entity.PlanRun {
	if todo.Kind == "" {
		return nil
	}
	run, ok := todo.Task.Job.Result().(entity.PlanRun)
	if !ok {
		return nil
	}
	return &run
}

func (m *manager) stats(all bool) []*entity.PlannerStats {
	result := make([]*entity.PlannerStats, 0)
	for _, todo := range m.todos {
		t := todo.Task.Job
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
		stats.Schedule = todo.Task.Schedule
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
	for _, todo := range m.stats(opts.All) {
		if selector.Empty() || selector.Matches(todo.Labels) {
			result = append(result, todo)
		}
	}
	return result, nil
}
