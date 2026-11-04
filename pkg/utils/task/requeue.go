package task

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/xhanio/framingo/pkg/utils/job"
)

// requeueLoop is the retry loop of one failed execution. Where the executor's
// retry runs the job again in place, holding its worker, this loop sends the task
// back through the queue, so the worker is free while the task cools down and the
// rerun is ordered like any other task.
type requeueLoop struct {
	ctx    context.Context
	cancel context.CancelFunc
	result chan error // the outcome of the rerun; buffered, the worker never waits for the loop
}

// settle closes an execution that was admitted, rerun being its requeue loop if it
// was run by one.
func (m *manager) settle(ctx, execCtx context.Context, reg *registration, rerun *requeueLoop, err error) {
	task := reg.task
	key := task.ID()
	m.el.Lock()
	defer m.el.Unlock()
	delete(m.running, task)
	reg.cancelRun = nil
	// what ended on a cancellation did not fail, and a job already running elsewhere
	// refused this start without failing
	failed := err != nil && task.Job.IsState(job.StateFailed) &&
		ctx.Err() == nil && execCtx.Err() == nil
	if rerun != nil {
		if err != nil && !failed {
			err = retry.Unrecoverable(err)
		}
		rerun.result <- err
		return
	}
	reg.requeues = 0
	if failed && task.RequeueAttempts > 0 && m.registered[key] == reg {
		loop := &requeueLoop{result: make(chan error, 1)}
		loop.ctx, loop.cancel = context.WithCancel(ctx)
		reg.loop = loop
		// safe against Stop's wait: ctx is not done under el, so the goroutine
		// ending the manager has yet to take el and wg is not at zero.
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.requeue(reg, loop, err)
		}()
		return
	}
	if task.isOneShot() && m.registered[key] == reg {
		delete(m.registered, key)
	}
}

// requeue runs the loop for failure, the outcome of the execution that started it.
func (m *manager) requeue(reg *registration, loop *requeueLoop, failure error) {
	defer loop.cancel()
	task := reg.task
	key := task.ID()
	if task.Ctx != nil {
		stop := context.AfterFunc(task.Ctx, loop.cancel)
		defer stop()
	}
	first := true
	err := retry.Do(
		func() error {
			if first {
				// the execution already ran: retry-go only has to take it from here
				first = false
				return failure
			}
			m.el.Lock()
			if err := loop.ctx.Err(); err != nil {
				m.el.Unlock()
				return retry.Unrecoverable(err)
			}
			reg.queued = true
			m.pq.Push(task) // under el: nothing pushes after Remove let go of it
			m.el.Unlock()
			select {
			case err := <-loop.result:
				return err
			case <-loop.ctx.Done():
				return retry.Unrecoverable(loop.ctx.Err())
			}
		},
		retry.Context(loop.ctx),
		// retry-go's attempts include the execution that started the loop
		retry.Attempts(uint(task.RequeueAttempts)+1),
		retry.LastErrorOnly(true),
		// retry-go asks for a delay only when another attempt follows
		retry.DelayType(func(n uint, err error, _ *retry.Config) time.Duration {
			m.el.Lock()
			defer m.el.Unlock()
			task.requeueing.Store(true)
			reg.requeues = n
			reg.cooling = time.Now().Add(task.RequeueCooldown)
			m.log.Debugf("task %s failed, requeue %d/%d in %s: %s", key, n, task.RequeueAttempts, task.RequeueCooldown, err)
			return task.RequeueCooldown
		}),
	)
	m.el.Lock()
	defer m.el.Unlock()
	if err != nil {
		m.log.Debugf("task %s ended its requeues with err: %s", key, err)
	} else {
		m.log.Debugf("task %s completed successfully after %d requeues", key, reg.requeues)
	}
	reg.loop = nil
	reg.cooling = time.Time{}
	current, registered := m.registered[key]
	if registered && current != reg {
		return // removed and added again: the queue and the task's state are the new registration's
	}
	if registered {
		reg.queued = false
		m.pq.Remove(task) // a rerun the loop was cut short before, while its state still holds
		if task.isOneShot() {
			delete(m.registered, key)
		}
	}
	task.requeueing.Store(false)
}
