package task

import (
	"context"
	"sync"

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
		m.el.Lock()
		if _, ok := m.registered[key]; ok {
			m.el.Unlock()
			return errors.Conflict.Newf("task %s already added", key)
		}
		reg := &registration{task: t, exec: newExecutor(t)}
		m.registered[key] = reg
		m.el.Unlock()
		if t.Schedule != "" {
			// scheduled by cron
			cronID, err := m.cm.AddFunc(t.Schedule, func() {
				m.pq.Push(t)
			})
			if err != nil {
				m.unregister(reg)
				return errors.Wrap(err)
			}
			m.cl.Lock()
			m.crons[key] = cronID
			m.cl.Unlock()
		} else {
			// run directly
			m.pq.Push(t)
		}
	}
	return nil
}

func (m *manager) Remove(tasks ...*Task) {
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
		if reg, ok := m.registered[key]; ok && reg.task == t {
			delete(m.registered, key)
		}
		m.el.Unlock()
		t.Job.Cancel()
		m.pq.Remove(t) // try removing anyway since task could be executing already
	}
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
					if !registered || reg.task != task || busy {
						m.el.Unlock()
						m.ew.Done()
						m.log.Debugf("task %s skipped: removed, or previous execution still in progress", key)
						return
					}
					m.running[task] = reg.exec
					m.el.Unlock()
					defer func() {
						if task.Schedule == "" {
							m.unregister(reg)
						}
						m.el.Lock()
						delete(m.running, task)
						m.el.Unlock()
						m.ew.Done() // unblock task queue before releasing the worker
					}()
					err := reg.exec.Start(task.Ctx, task.Params)
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

func (m *manager) Stats(id string) *executor.Stats {
	m.el.RLock()
	defer m.el.RUnlock()
	reg, ok := m.registered[id]
	if !ok {
		return nil
	}
	st := reg.exec.Stats()
	if st.Cooldown < 0 {
		st.Cooldown = 0
	}
	return st
}

// registration is a task added to the manager; its executor outlives a single execution so
// that cooldown, once and retry state carry over from one push to the next.
type registration struct {
	task *Task
	exec executor.Executor
}

func (m *manager) unregister(reg *registration) {
	m.el.Lock()
	defer m.el.Unlock()
	if key := reg.task.ID(); m.registered[key] == reg {
		delete(m.registered, key)
	}
}

func newExecutor(t *Task) executor.Executor {
	opts := []executor.Option{
		executor.WithTimeout(t.Timeout),
		executor.WithRetry(t.RetryAttempts, t.RetryDelay),
		executor.WithCooldown(t.Cooldown),
	}
	if t.Once {
		opts = append(opts, executor.Once())
	}
	return executor.New(t.Job, opts...)
}
