package task

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
	"github.com/xhanio/framingo/pkg/utils/log"
)

var exiting = &Task{}

var _ Manager = (*manager)(nil)

type manager struct {
	log log.Logger

	name string

	cm    *cron.Cron
	cl    *sync.RWMutex // lock for crons
	crons map[string]cron.EntryID

	pq   staque.Priority[*Task]
	pipe chan *Task

	concurrent int
	workers    chan struct{}
	el         *sync.RWMutex   // lock for registered and running
	ew         *sync.WaitGroup // wait group for executing
	registered map[string]*registration
	running    map[*Task]executor.Executor
	ended      map[*Task]chan struct{} // closed as a removed task's execution settles

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func New(opts ...Option) Manager {
	return newScheduler(opts...)
}

func newScheduler(opts ...Option) *manager {
	m := &manager{
		cl:         &sync.RWMutex{},
		crons:      make(map[string]cron.EntryID),
		el:         &sync.RWMutex{},
		ew:         &sync.WaitGroup{},
		registered: make(map[string]*registration),
		running:    make(map[*Task]executor.Executor),
		ended:      make(map[*Task]chan struct{}),
		wg:         &sync.WaitGroup{},
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.pq = staque.NewPriority(
		staque.WithLessFunc(priorityFunc),
		staque.WithLogger[*Task](m.log),
		staque.BlockIfEmpty[*Task](),
	)
	m.pipe = make(chan *Task)
	m.workers = make(chan struct{}, m.concurrent)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Add(tasks ...*Task) error {
	for _, t := range tasks {
		key := t.ID()
		if key == "" {
			continue
		}
		if t.Manual && t.Schedule != "" {
			return errors.InvalidArgument.Newf("task %s cannot be both manual and scheduled", key)
		}
		m.el.Lock()
		if _, ok := m.registered[key]; ok {
			m.el.Unlock()
			return errors.Conflict.Newf("task %s already added", key)
		}
		reg := &registration{task: t, exec: newExecutor(t)}
		m.registered[key] = reg
		if t.isOneShot() {
			// run directly
			reg.queued = true
			m.pq.Push(t)
		}
		m.el.Unlock()
		if t.Schedule != "" {
			// scheduled by cron
			cronID, err := m.cm.AddFunc(t.Schedule, func() {
				m.tick(reg)
			})
			if err != nil {
				m.unregister(reg)
				return errors.Wrap(err)
			}
			m.cl.Lock()
			m.crons[key] = cronID
			m.cl.Unlock()
		}
	}
	return nil
}

func (m *manager) Remove(tasks ...*Task) <-chan struct{} {
	var going []chan struct{}
	for _, t := range tasks {
		key := t.ID()
		if key == "" {
			continue
		}
		if t.Schedule != "" {
			m.cl.Lock()
			if cid, ok := m.crons[key]; ok {
				m.cm.Remove(cid)
				delete(m.crons, key)
			}
			m.cl.Unlock()
		}
		m.el.Lock()
		// try removing anyway since task could be executing already; first, since
		// ending its requeue changes what the queue orders it by
		m.pq.Remove(t)
		var cancelRun context.CancelFunc
		if reg, ok := m.registered[key]; ok && reg.task == t {
			delete(m.registered, key)
			if reg.loop != nil {
				reg.loop.cancel()
			}
			cancelRun = reg.cancelRun
		}
		// admission happens under el too, so an execution is either here or never starts
		if _, running := m.running[t]; running {
			ended, ok := m.ended[t]
			if !ok {
				ended = make(chan struct{})
				m.ended[t] = ended
			}
			going = append(going, ended)
		}
		m.el.Unlock()
		t.Job.Cancel()
		if cancelRun != nil {
			cancelRun()
		}
	}
	return allClosed(going)
}

func allClosed(chans []chan struct{}) <-chan struct{} {
	if len(chans) == 1 {
		return chans[0]
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, ch := range chans {
			<-ch
		}
	}()
	return done
}

func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("service already started")
		return nil
	}
	m.cm.Start()
	m.pipe = make(chan *Task)
	m.workers = make(chan struct{}, m.concurrent)
	m.ctx, m.cancel = context.WithCancel(ctx)
	runCtx := m.ctx // a worker can outlive this run, and m.ctx with it
	m.wg.Add(2)
	// goroutine to fetch tasks
	go func() {
		defer m.wg.Done()
		for {
			task, _ := m.pq.Pop()
			if task == exiting {
				// keep the exiting loop until receiving cancel message from m.ctx.Done()
				m.pq.Push(exiting)
			} else if !task.IsValid() {
				continue
			}
			if task.Exclusive {
				m.log.Debugf("task %s wait for all other tasks to complete...", task.ID())
				m.ew.Wait()
				m.log.Debugf("continue on current exclusive task %s...", task.ID())
			}
			// before the handoff: a quick worker may call Done as soon as it receives
			m.ew.Add(1)
			select {
			case <-m.ctx.Done():
				m.ew.Done()
				// Close only pipe: this goroutine is its sole sender, so
				// closing here cannot race a send, and workers reading it get
				// a nil task that IsValid() rejects.
				//
				// workers is NOT closed here. Its only sender is the executor
				// goroutine below, which selects on ctx.Done() at the same
				// moment - closing it from this side races that send, and a
				// send on a closed channel panics rather than merely tripping
				// the race detector. Nothing ranges over workers, so leaving it
				// to the GC is sufficient.
				close(m.pipe)
				m.pq.Reset()
				m.log.Infof("stopped fetching execution tasks")
				return
			case m.pipe <- task:
			}
			if task.Exclusive {
				// block all other tasks from being popped
				m.log.Debugf("wait for current exclusive task %s to complete...", task.ID())
				m.ew.Wait()
				m.log.Debug("continue popping other tasks...")
			}
		}
	}()
	// goroutine to execute tasks concurrently
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				// clear all cron tasks and stop cron manager
				m.cl.Lock()
				defer m.cl.Unlock()
				for key, cid := range m.crons {
					m.cm.Remove(cid)
					delete(m.crons, key)
				}
				m.cm.Stop()
				// push a task with a nil task to unblock m.pq.Pop() and enter the exiting loop above
				m.pq.Push(exiting)
				// cancel all currently executing tasks
				m.el.Lock()
				defer m.el.Unlock()
				for _, te := range m.running {
					_ = te.Stop(false)
				}
				for _, reg := range m.registered {
					if reg.cancelRun != nil {
						reg.cancelRun() // after the job's cancel, so a run ends canceled, not failed
					}
				}
				clear(m.registered)
				m.log.Infof("stopped executing tasks")
				return
			case m.workers <- struct{}{}:
				go func() {
					task, ok := <-m.pipe
					defer func() {
						<-m.workers
					}()
					if !ok {
						return
					}
					if !task.IsValid() {
						m.ew.Done()
						return
					}
					key := task.ID()
					m.log.Debugf("task %s received", key)
					m.el.Lock()
					reg, registered := m.registered[key]
					_, busy := m.running[task]
					if !registered || reg.task != task || busy || !reg.queued {
						m.el.Unlock()
						m.ew.Done()
						m.log.Debugf("task %s skipped: removed, canceled, or previous execution still in progress", key)
						return
					}
					reg.queued = false
					m.pq.Remove(task) // a copy from a Remove and Add during the handoff; see fire
					task.requeueing.Store(false)
					m.running[task] = reg.exec
					rerun := reg.loop
					params := task.Params
					if reg.params != nil {
						params = reg.params
					}
					parent := task.Ctx
					if parent == nil {
						parent = context.Background()
					}
					execCtx, cancel := context.WithCancel(parent)
					reg.cancelRun = cancel
					m.el.Unlock()
					var err error
					defer func() {
						m.settle(runCtx, execCtx, reg, rerun, err)
						cancel()
						m.ew.Done() // unblock task queue before releasing the worker
					}()
					err = reg.exec.Start(execCtx, params)
					if err != nil {
						m.log.Debugf("task %s ended with err: %s", task.ID(), err)
					} else {
						m.log.Debugf("task %s completed successfully", task.ID())
					}
				}()
			}
		}
	}()
	return nil
}

func (m *manager) Stop(wait bool) error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	if wait {
		m.wg.Wait()
	}
	m.cancel = nil
	return nil
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
	_, running := m.running[task]
	if !running && !reg.queued && reg.loop == nil {
		m.el.Unlock()
		return errors.Conflict.Newf("task %s is not running", id)
	}
	if reg.queued {
		m.pq.Remove(task) // first: ending a requeue changes what the queue orders it by
		reg.queued = false
		if !running && reg.loop == nil && task.isOneShot() {
			delete(m.registered, id)
		}
	}
	if reg.loop != nil {
		reg.loop.cancel()
	}
	cancelRun := reg.cancelRun
	m.el.Unlock()
	if running {
		task.Job.Cancel()
	}
	if cancelRun != nil {
		cancelRun()
	}
	return nil
}

func (m *manager) Stats(id string) *Stats {
	m.el.RLock()
	defer m.el.RUnlock()
	reg, ok := m.registered[id]
	if !ok {
		return nil
	}
	st := &Stats{Stats: *reg.exec.Stats(), Requeues: reg.requeues}
	if left := time.Until(reg.cooling); left > 0 {
		st.Cooldown = left
	}
	return st
}

// registration is a task added to the manager; its executor outlives a single execution so
// that retry state carries over from one push to the next.
type registration struct {
	task *Task
	exec executor.Executor

	// guarded by the manager's el
	queued    bool               // in the queue, or held on its way to a worker
	params    any                // what Trigger gave this firing, in place of Task.Params
	cancelRun context.CancelFunc // ends the current execution, its retry waits included
	loop      *requeueLoop       // set while a failed execution is being requeued
	requeues  uint               // times the latest execution went back to the queue
	cooling   time.Time          // when the current RequeueCooldown ends
}

// fire queues a registered task unless it is queued, running or being requeued. The queue
// orders by state, so a task must be out of it while its state changes: a queue out of
// order cannot find the task again, and removing it panics.
func (m *manager) fire(reg *registration, params any) error {
	key := reg.task.ID()
	m.el.Lock()
	defer m.el.Unlock()
	if m.registered[key] != reg {
		return errors.NotFound.Newf("task %s not found", key)
	}
	if _, running := m.running[reg.task]; running || reg.queued || reg.loop != nil {
		return errors.Conflict.Newf("task %s is already queued, running or being requeued", key)
	}
	reg.queued = true
	reg.params = params
	m.pq.Push(reg.task)
	return nil
}

func (m *manager) tick(reg *registration) {
	if err := m.fire(reg, nil); err != nil {
		m.log.Debugf("tick skipped: %s", err)
	}
}

func (m *manager) unregister(reg *registration) {
	m.el.Lock()
	defer m.el.Unlock()
	if key := reg.task.ID(); m.registered[key] == reg {
		delete(m.registered, key)
	}
}

func newExecutor(t *Task) executor.Executor {
	return executor.New(t.Job,
		executor.WithTimeout(t.Timeout),
		executor.WithRetry(t.RetryAttempts, t.RetryDelay),
	)
}
