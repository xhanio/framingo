package planner

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/google/uuid"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/task"
)

const timeFormat = "2006-01-02 15:04:05.00"

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	es common.EventSender

	tm task.Manager

	sync.RWMutex
	todos map[string]*TODO
}

func New(es common.EventSender, opts ...Option) Manager {
	return newManager(es, opts...)
}

func newManager(es common.EventSender, opts ...Option) *manager {
	m := &manager{
		log:   log.Default,
		es:    es,
		todos: make(map[string]*TODO),
	}
	for _, opt := range opts {
		opt(m)
	}
	m.log = m.log.By(m)
	m.tm = task.New(
		task.MaxConcurrency(10),
		task.WithLogger(m.log),
	)
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Add(todo *TODO) error {
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

func (m *manager) stats(all bool) []*Stats {
	result := make([]*Stats, 0)
	for _, todo := range m.todos {
		t := todo.Task.Job
		if !all && t.State() == job.StateSucceeded {
			continue
		}
		ts := t.Stats()
		stats := &Stats{
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

func (m *manager) Stats(opts StatsOptions) ([]*Stats, error) {
	selector, err := labels.Parse(opts.Selector)
	if err != nil {
		return nil, errors.InvalidArgument.Wrap(err)
	}
	m.RLock()
	defer m.RUnlock()
	var result []*Stats
	for _, todo := range m.stats(opts.All) {
		if selector.Empty() || selector.Matches(todo.Labels) {
			result = append(result, todo)
		}
	}
	return result, nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	if debug {
		pt := printutil.NewTable(w)
		pt.Header(m.Name())
		pt.Title("ID", "Schedule", "StartedAt", "State", "Error", "Labels")
		for _, t := range m.stats(debug) {
			start := ""
			if !t.StartedAt.IsZero() {
				start = t.StartedAt.Local().Format(timeFormat)
			}
			if t.ExecutionTime > 0 {
				start += fmt.Sprintf(" (%s)", t.ExecutionTime.Round(time.Millisecond).String())
			}
			state := string(t.State)
			if t.Cooldown > 0 {
				state += fmt.Sprintf(" (cd:%s)", t.Cooldown.Round(time.Second).String())
			}
			pt.Row(t.ID, t.Schedule, start, state, t.Error, t.Labels.String())
		}
		pt.NewLine()
		pt.Flush()
	}
}

func (m *manager) Start(ctx context.Context) error {
	err := m.tm.Start(ctx)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	var errs []error
	errs = append(errs, m.tm.Stop(wait))
	return errors.Combine(errs...)
}
