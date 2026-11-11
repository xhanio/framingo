package task

import (
	"context"

	"github.com/xhanio/errors"
	"golang.org/x/sync/semaphore"
)

// Start can come right after any Stop, while what that Stop left still runs.
func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("service already started")
		return nil
	}
	ctx, m.cancel = context.WithCancel(ctx)
	looped := make(chan struct{})
	m.looped = looped
	m.cm.Start()
	go func() {
		defer close(looped)
		m.loop(ctx, semaphore.NewWeighted(int64(m.concurrent)))
	}()
	return nil
}

// loop runs the queue in slots of its own Start, so that what a forced stop leaves running
// takes none from the next.
func (m *manager) loop(ctx context.Context, slots *semaphore.Weighted) {
	defer m.log.Infof("stopped executing tasks")
	for slots.Acquire(ctx, 1) == nil {
		reg := m.pop(ctx)
		if reg == nil {
			return
		}
		held := int64(1)
		if reg.exclusive() {
			// every slot, so that it runs alone
			if slots.Acquire(ctx, int64(m.concurrent)-1) != nil {
				return
			}
			held = int64(m.concurrent)
		}
		run := m.admit(ctx, reg)
		if run == nil {
			slots.Release(held)
			continue
		}
		m.executions.Go(func() {
			defer slots.Release(held)
			run()
		})
	}
}

// pop waits for a registration in the queue, and returns nil once ctx ends.
func (m *manager) pop(ctx context.Context) *registration {
	for {
		reg, _ := m.pq.Pop() // nil when woken on an empty queue
		if ctx.Err() != nil {
			return nil
		}
		if reg.valid() {
			return reg
		}
	}
}

// admit returns what runs reg, or nil if it was removed or canceled since it was queued, or if
// its task still runs from before, which it then waits for.
func (m *manager) admit(ctx context.Context, reg *registration) func() {
	task := reg.task
	key := task.ID()
	m.el.Lock()
	defer m.el.Unlock()
	if m.registered[key] != reg || !reg.queued {
		m.log.Debugf("task %s skipped: removed or canceled", key)
		return nil
	}
	if e, busy := m.running[task.Job]; busy {
		// from before a Remove or a forced stop: a job runs once at a time
		e.next = reg
		m.log.Debugf("task %s waits for its last execution to end", key)
		return nil
	}
	reg.queued = false
	m.pq.Remove(reg) // queued again too if it was canceled and triggered while popped
	if reg.unwatch != nil {
		reg.unwatch() // the rerun's own context comes from Task.Ctx
		reg.unwatch = nil
	}
	params := task.Params
	if reg.params != nil {
		params = reg.params
	}
	parent := task.Ctx
	if parent == nil {
		parent = context.Background()
	}
	execCtx, cancel := context.WithCancel(parent)
	e := &execution{reg: reg, cancel: cancel, done: make(chan struct{})}
	m.running[task.Job] = e
	return func() {
		defer cancel()
		err := reg.exec.Start(execCtx, params)
		if err != nil {
			m.log.Debugf("task %s ended with err: %s", key, err)
		} else {
			m.log.Debugf("task %s completed successfully", key)
		}
		m.settle(ctx, execCtx, e, err)
	}
}

// Stop ends the loop, forgets the tasks and cancels the executions, then waits for them, those an
// earlier Stop left included, until ctx ends. What still runs then is sent SignalKill and left
// to end beside whatever Start comes next.
func (m *manager) Stop(ctx context.Context) error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
		m.pq.Push(m.wake)
		<-m.looped
		m.pq.Remove(m.wake) // left if the loop returned without popping it
		m.halt()
	}
	err := m.executions.Wait(ctx)
	if err == nil {
		return nil
	}
	if left := m.kill(); left > 0 {
		return errors.Wrapf(err, "%s stopped with %d executions still going", m.name, left)
	}
	return nil
}

func (m *manager) halt() {
	m.el.Lock()
	defer m.el.Unlock()
	for _, e := range m.running {
		_ = e.reg.exec.Stop(false)
		e.cancel() // after the job's cancel, so a run ends canceled, not failed
	}
	for _, reg := range m.registered {
		m.drop(reg)
	}
	clear(m.registered)
	m.cm.Stop()
}

// kill sends SignalKill to the executions still going, and says how many there are.
func (m *manager) kill() int {
	m.el.RLock()
	going := make([]*execution, 0, len(m.running))
	for _, e := range m.running {
		going = append(going, e)
	}
	m.el.RUnlock()
	return killGoing(going)
}
