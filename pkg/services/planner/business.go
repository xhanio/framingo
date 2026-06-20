package planner

import (
	"sort"

	"github.com/google/uuid"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *manager) Add(todo *entity.Plan) error {
	if todo.ID == "" {
		todo.ID = uuid.NewString()
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

func (m *manager) Cancel(id string) error {
	m.RLock()
	defer m.RUnlock()
	if todo, ok := m.todos[id]; ok {
		todo.Task.Job.Cancel()
		return nil
	}
	return errors.NotFound.Newf("failed to cancel todo %s: todo id not found", id)
}

func (m *manager) Delete(id string, force bool) error {
	m.Lock()
	defer m.Unlock()
	todo, ok := m.todos[id]
	if ok {
		m.tm.Remove(todo.Task)
		delete(m.todos, id)
		return nil
	}
	return errors.NotFound.Newf("todo id %s not found", id)
}

func (m *manager) GetResult(id string) (any, error) {
	m.RLock()
	defer m.RUnlock()
	if todo, ok := m.todos[id]; ok {
		return todo.Task.Job.Result(), nil
	}
	return nil, errors.NotFound.Newf("failed to get result of todo %s: todo id not found", id)
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
