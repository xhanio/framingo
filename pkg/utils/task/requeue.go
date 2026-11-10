package task

import (
	"context"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
)

// settle closes an admitted execution. A failure with requeues left cools down without
// holding a worker, then goes back to the queue as a rerun of the same firing.
func (m *manager) settle(ctx, execCtx context.Context, e *execution, err error) {
	reg := e.reg
	task := reg.task
	key := task.ID()
	m.el.Lock()
	defer m.el.Unlock()
	delete(m.running, task)
	close(e.done)
	current := m.registered[key] == reg
	// what ended on a cancellation did not fail, and a job already running elsewhere
	// refused this start without failing
	failed := err != nil && task.Job.IsState(job.StateFailed) &&
		ctx.Err() == nil && execCtx.Err() == nil
	if failed && current && reg.requeues < uint(task.RequeueAttempts) {
		m.startCooldown(reg, err)
		return
	}
	if task.isOneShot() && current {
		delete(m.registered, key)
	}
}

// The caller holds el.
func (m *manager) startCooldown(reg *registration, failure error) {
	task := reg.task
	reg.requeues++
	reg.requeueing.Store(true) // its execution just ended, so it is out of the queue
	reg.cooling = time.Now().Add(task.RequeueCooldown)
	reg.cooldown = time.AfterFunc(task.RequeueCooldown, func() { m.cooledDown(reg) })
	if task.Ctx != nil {
		reg.unwatch = context.AfterFunc(task.Ctx, func() { m.contextEnded(reg) })
	}
	m.log.Debugf("task %s failed, requeue %d/%d in %s: %s", task.ID(), reg.requeues, task.RequeueAttempts, task.RequeueCooldown, failure)
}

func (m *manager) cooledDown(reg *registration) {
	m.el.Lock()
	defer m.el.Unlock()
	if reg.cooldown == nil || m.registered[reg.task.ID()] != reg {
		return // the wait was stopped as the timer fired
	}
	reg.cooldown, reg.cooling = nil, time.Time{}
	reg.queued = true
	m.pq.Push(reg)
}

func (m *manager) contextEnded(reg *registration) {
	m.el.Lock()
	defer m.el.Unlock()
	if !reg.requeueing.Load() {
		return // the rerun started, or the wait was stopped already
	}
	m.stopWaiting(reg)
	if key := reg.task.ID(); reg.task.isOneShot() && m.registered[key] == reg {
		delete(m.registered, key)
	}
}

// stopWaiting ends a wait to run, cooling down or in the queue. The caller holds el.
func (m *manager) stopWaiting(reg *registration) {
	if reg.cooldown != nil {
		reg.cooldown.Stop()
		reg.cooldown, reg.cooling = nil, time.Time{}
	}
	if reg.unwatch != nil {
		reg.unwatch()
		reg.unwatch = nil
	}
	if reg.queued {
		m.pq.Remove(reg) // first: the queue orders by requeueing
		reg.queued = false
	}
	reg.requeueing.Store(false)
}
