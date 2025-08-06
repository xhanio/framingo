package scheduler

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"xhanio/framingo/pkg/types/common"
	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/infra"
	"xhanio/framingo/pkg/util/log"
	"xhanio/framingo/pkg/util/queue"
	"xhanio/framingo/pkg/util/reflectutil"
	"xhanio/framingo/pkg/util/task/executor"
)

var exiting = &Plan{}

var _ Scheduler = (*scheduler)(nil)

type scheduler struct {
	log log.Logger

	name string

	cm    *cron.Cron
	tz    *time.Location
	cl    *sync.RWMutex // lock for crons
	crons map[string]cron.EntryID

	pq   queue.PriorityQueue[*Plan]
	pipe chan *Plan

	concurrent int
	workers    chan struct{}
	el         *sync.RWMutex   // lock for executing
	ew         *sync.WaitGroup // wait group for executing
	executing  map[string]executor.Executor

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func New(opts ...Option) Scheduler {
	return newScheduler(opts...)
}

func newScheduler(opts ...Option) *scheduler {
	s := &scheduler{
		cl:        &sync.RWMutex{},
		crons:     make(map[string]cron.EntryID),
		el:        &sync.RWMutex{},
		ew:        &sync.WaitGroup{},
		executing: make(map[string]executor.Executor),
		wg:        &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = log.Default
	}
	if s.tz == nil {
		s.tz = infra.Timezone
	}
	s.cm = cron.New(
		cron.WithLocation(s.tz),
		cron.WithParser(cron.NewParser(cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)),
	)
	s.pq = queue.NewPriority(
		queue.WithLessFunc(priorityFunc),
		queue.WithLogger[*Plan](s.log),
		queue.BlockIfEmpty[*Plan](),
	)
	s.pipe = make(chan *Plan, s.concurrent)
	s.workers = make(chan struct{}, s.concurrent)
	return s
}

func (s *scheduler) Name() string {
	if s.name == "" {
		s.name = path.Join(reflectutil.Locate(s))
	}
	return s.name
}

func (s *scheduler) Dependencies() []common.Service {
	return nil
}

func (s *scheduler) Add(plans ...*Plan) error {
	for _, p := range plans {
		key := p.Key()
		if key == "" {
			continue
		}
		if p.Schedule != "" {
			// scheduled by cron
			cronID, err := s.cm.AddFunc(p.Schedule, func() {
				s.pq.Push(p)
			})
			if err != nil {
				return errors.Wrap(err)
			}
			s.cl.Lock()
			s.crons[key] = cronID
			s.cl.Unlock()
		} else {
			// run directly
			s.pq.Push(p)
		}
	}
	return nil
}

func (s *scheduler) Remove(plans ...*Plan) {
	for _, p := range plans {
		key := p.Key()
		if key == "" {
			continue
		}
		if p.Schedule != "" {
			s.cl.Lock()
			if cid, ok := s.crons[key]; ok {
				s.cm.Remove(cid)
				delete(s.crons, key)
			}
			s.cl.Unlock()
		}
		p.Task.Cancel()
		s.pq.Remove(p) // try removing anyway since task could be executing already
	}
}

func (s *scheduler) Start(ctx context.Context) error {
	if s.cancel != nil {
		s.log.Warnf("service already started")
		return nil
	}
	s.cm.Start()
	s.pipe = make(chan *Plan)
	s.workers = make(chan struct{}, s.concurrent)
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(2)
	// goroutine to fetch task plans
	go func() {
		defer s.wg.Done()
		for {
			plan, _ := s.pq.Pop()
			if plan == exiting {
				// keep the exiting loop until receiving cancel message from s.ctx.Done()
				s.pq.Push(exiting)
			} else if !plan.IsValid() {
				continue
			}
			if plan.Exclusive {
				s.log.Debug("wait for all other plans to complete...")
				s.ew.Wait()
				s.log.Debugf("continue on current exclusive plan %s...", plan.Task.ID())
			}
			select {
			case <-s.ctx.Done():
				close(s.pipe)
				close(s.workers)
				s.pq.Reset()
				s.log.Infof("stopped fetching execution plans")
				return
			case s.pipe <- plan:
				s.ew.Add(1)
			}
			if plan.Exclusive {
				// block all other plans from being popped
				s.log.Debugf("wait for current exclusive plan %s to complete...", plan.Task.ID())
				s.ew.Wait()
				s.log.Debug("continue popping other plans...")
			}
		}
	}()
	// goroutine to execute tasks concurrently
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ctx.Done():
				// clear all cron tasks and stop cron manager
				s.cl.Lock()
				defer s.cl.Unlock()
				for key, cid := range s.crons {
					s.cm.Remove(cid)
					delete(s.crons, key)
				}
				s.cm.Stop()
				// push a plan with a nil task to unblock s.pq.Pop() and enter the exiting loop above
				s.pq.Push(exiting)
				// cancel all currently executing tasks
				s.el.RLock()
				defer s.el.RUnlock()
				for _, je := range s.executing {
					_ = je.Stop(false)
				}
				s.log.Infof("stopped executing tasks")
				return
			case s.workers <- struct{}{}:
				go func() {
					defer s.ew.Done()
					plan := <-s.pipe
					if !plan.IsValid() {
						return
					}
					defer func(plan *Plan) {
						s.el.Lock()
						delete(s.executing, plan.Task.ID())
						s.el.Unlock()
						if plan.Task.Err() != nil {
							s.log.Debugf("task %s ended with err: %s", plan.Task.ID(), plan.Task.Err())
						} else {
							s.log.Debugf("task %s completed succseefully", plan.Task.ID())
						}
						<-s.workers
					}(plan)
					s.log.Debugf("plan of task %s received", plan.Task.ID())
					je := executor.New(plan.Task, plan.Opts...)
					s.el.Lock()
					s.executing[plan.Key()] = je
					s.el.Unlock()
					_ = je.Start(plan.Ctx)
				}()
			}
		}
	}()
	return nil
}

func (s *scheduler) Stop(wait bool) error {
	if s.cancel == nil {
		return nil
	}
	s.cancel()
	if wait {
		s.wg.Wait()
	}
	s.cancel = nil
	return nil
}

func (s *scheduler) Stats(id string) *executor.Stats {
	s.el.RLock()
	defer s.el.RUnlock()
	if je, ok := s.executing[id]; ok {
		return je.Stats()
	}
	return nil
}
