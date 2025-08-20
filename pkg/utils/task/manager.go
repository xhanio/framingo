package task

import (
	"context"
	"path"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/infra"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
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
	el         *sync.RWMutex   // lock for executing
	ew         *sync.WaitGroup // wait group for executing
	executing  map[string]executor.Executor

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func New(opts ...Option) Manager {
	return newScheduler(opts...)
}

func newScheduler(opts ...Option) *manager {
	m := &manager{
		cl:        &sync.RWMutex{},
		crons:     make(map[string]cron.EntryID),
		el:        &sync.RWMutex{},
		ew:        &sync.WaitGroup{},
		executing: make(map[string]executor.Executor),
		wg:        &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	if m.cm == nil {
		m.cm = cron.New(
			cron.WithLocation(infra.Timezone),
			cron.WithParser(cron.NewParser(cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)),
		)
	}
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
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Add(tasks ...*Task) error {
	for _, t := range tasks {
		key := t.Key()
		if key == "" {
			continue
		}
		if t.Schedule != "" {
			// scheduled by cron
			cronID, err := m.cm.AddFunc(t.Schedule, func() {
				m.pq.Push(t)
			})
			if err != nil {
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
		key := t.Key()
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
				m.log.Debugf("task %s wait for all other tasks to complete...", task.Key())
				m.ew.Wait()
				m.log.Debugf("continue on current exclusive task %s...", task.Key())
			}
			select {
			case <-m.ctx.Done():
				close(m.pipe)
				close(m.workers)
				m.pq.Reset()
				m.log.Infof("stopped fetching execution tasks")
				return
			case m.pipe <- task:
				m.ew.Add(1)
			}
			if task.Exclusive {
				// block all other tasks from being popped
				m.log.Debugf("wait for current exclusive task %s to complete...", task.Key())
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
				m.el.RLock()
				defer m.el.RUnlock()
				for _, te := range m.executing {
					_ = te.Stop(false)
				}
				m.log.Infof("stopped executing tasks")
				return
			case m.workers <- struct{}{}:
				go func() {
					task := <-m.pipe
					defer func() {
						<-m.workers
					}()
					if !task.IsValid() {
						return
					}
					defer func(task *Task) {
						m.el.Lock()
						delete(m.executing, task.Key())
						m.el.Unlock()
						m.ew.Done() // unblock task queue before releasing the worker
					}(task)
					m.log.Debugf("task %s received", task.Key())
					var opts []executor.Option
					if task.Once {
						opts = append(opts, executor.Once())
					}
					opts = append(opts, executor.WithTimeout(task.Timeout))
					opts = append(opts, executor.WithRetry(task.RetryAttempts, task.RetryDelay))
					opts = append(opts, executor.WithCooldown(task.Cooldown))
					te := executor.New(task.Job, opts...)
					m.el.Lock()
					m.executing[task.Key()] = te
					m.el.Unlock()
					err := te.Start(task.Ctx, task.Params)
					if err != nil {
						m.log.Debugf("task %s ended with err: %s", task.Key(), err)
					} else {
						m.log.Debugf("task %s completed successfully", task.Key())
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
	if te, ok := m.executing[id]; ok {
		return te.Stats()
	}
	return nil
}
