package task

import (
	"context"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

func (m *manager) Add(tasks ...*entity.Task) error {
	for _, t := range tasks {
		if t.ID() == "" {
			continue
		}
		if err := m.add(t); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) add(t *entity.Task) error {
	key := t.ID()
	if t.Manual && t.Schedule != "" {
		return errors.InvalidArgument.Newf("task %s cannot be both manual and scheduled", key)
	}
	m.el.Lock()
	defer m.el.Unlock()
	if _, ok := m.registered[key]; ok {
		return errors.Conflict.Newf("task %s already added", key)
	}
	reg := &registration{task: t, exec: newExecutor(t), priority: t.Priority}
	if t.Schedule != "" {
		// under el, so no Remove comes between the task and its schedule
		id, err := m.cm.AddFunc(t.Schedule, func() { m.tick(reg) })
		if err != nil {
			return errors.Wrap(err)
		}
		reg.cron = id
	}
	m.registered[key] = reg
	if t.IsOneShot() {
		m.enqueue(reg)
	}
	return nil
}

func (m *manager) Remove(ctx context.Context, tasks ...*entity.Task) error {
	var going []*execution
	for _, t := range tasks {
		key := t.ID()
		if key == "" {
			continue
		}
		m.el.Lock()
		if reg, ok := m.registered[key]; ok && reg.task == t {
			delete(m.registered, key)
			m.drop(reg)
		}
		// admission happens under el too, so an execution is either here or never starts
		e, running := m.running[t]
		if running {
			going = append(going, e)
		}
		m.el.Unlock()
		t.Job.Cancel()
		if running {
			e.cancel()
		}
	}
	for _, e := range going {
		select {
		case <-e.done:
		case <-ctx.Done():
			if left := killGoing(going); left > 0 {
				return errors.Wrapf(ctx.Err(), "%d removed executions still going", left)
			}
			return nil
		}
	}
	return nil
}

// killGoing sends SignalKill to those of the executions still going, and says how many there are.
func killGoing(executions []*execution) int {
	var left int
	for _, e := range executions {
		select {
		case <-e.done:
		default:
			e.reg.task.Job.Signal(job.SignalKill)
			left++
		}
	}
	return left
}

// Trigger queues the registered task to run now, with params in place of Task.Params for
// this execution and its requeues; nil keeps Task.Params. A task already queued, running or
// being requeued is a Conflict.
func (m *manager) Trigger(id string, params any) error {
	m.el.RLock()
	reg, ok := m.registered[id]
	m.el.RUnlock()
	if !ok {
		return errors.NotFound.Newf("task %s not found", id)
	}
	return m.fire(reg, params)
}

// Cancel ends what the task is doing, whether queued, running, waiting for a retry or being
// requeued, and keeps it registered unless it is one-shot. A task doing none of those is a
// Conflict.
func (m *manager) Cancel(id string) error {
	m.el.Lock()
	reg, ok := m.registered[id]
	if !ok {
		m.el.Unlock()
		return errors.NotFound.Newf("task %s not found", id)
	}
	task := reg.task
	e, running := m.running[task]
	if !running && !reg.queued && reg.cooldown == nil {
		m.el.Unlock()
		return errors.Conflict.Newf("task %s is not running", id)
	}
	if reg.queued || reg.cooldown != nil {
		m.stopWaiting(reg)
		if task.IsOneShot() {
			delete(m.registered, id)
		}
	}
	m.el.Unlock()
	if running {
		task.Job.Cancel()
		e.cancel()
	}
	return nil
}

func (m *manager) Stats(id string) *entity.TaskStats {
	m.el.RLock()
	defer m.el.RUnlock()
	reg, ok := m.registered[id]
	if !ok {
		return nil
	}
	st := &entity.TaskStats{Stats: *reg.exec.Stats(), Requeues: reg.requeues, Requeueing: reg.requeueing()}
	if left := time.Until(reg.cooling); left > 0 {
		st.Cooldown = left
	}
	return st
}

func (m *manager) fire(reg *registration, params any) error {
	key := reg.task.ID()
	m.el.Lock()
	defer m.el.Unlock()
	if m.registered[key] != reg {
		return errors.NotFound.Newf("task %s not found", key)
	}
	if _, running := m.running[reg.task]; running || reg.queued || reg.cooldown != nil {
		return errors.Conflict.Newf("task %s is already queued, running or cooling down", key)
	}
	reg.params = params
	reg.requeues = 0
	m.enqueue(reg)
	return nil
}

// The caller holds el.
func (m *manager) enqueue(reg *registration) {
	m.arrivals++
	reg.seq = m.arrivals
	reg.queued = true
	m.pq.Push(reg)
}

// drop takes a registration's schedule and waits out of the manager. The caller holds el.
func (m *manager) drop(reg *registration) {
	if reg.cron != 0 {
		m.cm.Remove(reg.cron)
		reg.cron = 0
	}
	m.stopWaiting(reg)
}

func (m *manager) tick(reg *registration) {
	if err := m.fire(reg, nil); err != nil {
		m.log.Debugf("tick skipped: %s", err)
	}
}

func newExecutor(t *entity.Task) executor.Executor {
	return executor.New(t.Job,
		executor.WithTimeout(t.Timeout),
		executor.WithRetry(t.RetryAttempts, t.RetryDelay),
	)
}
