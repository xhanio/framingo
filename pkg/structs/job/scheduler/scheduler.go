package scheduler

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"xhanio/framingo/pkg/structs/job/executor"
	"xhanio/framingo/pkg/structs/queue"
	"xhanio/framingo/pkg/types/common"
	"xhanio/framingo/pkg/utils/errors"
	"xhanio/framingo/pkg/utils/infra"
	"xhanio/framingo/pkg/utils/log"
	"xhanio/framingo/pkg/utils/reflectutil"
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
		p.Job.Cancel()
		s.pq.Remove(p) // try removing anyway since job could be executing already
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
	// goroutine to fetch job plans
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
				s.log.Debugf("continue on current exclusive plan %s...", plan.Job.ID())
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
				s.log.Debugf("wait for current exclusive plan %s to complete...", plan.Job.ID())
				s.ew.Wait()
				s.log.Debug("continue popping other plans...")
			}
		}
	}()
	// goroutine to execute jobs concurrently
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ctx.Done():
				// clear all cron jobs and stop cron manager
				s.cl.Lock()
				defer s.cl.Unlock()
				for key, cid := range s.crons {
					s.cm.Remove(cid)
					delete(s.crons, key)
				}
				s.cm.Stop()
				// push a plan with a nil job to unblock s.pq.Pop() and enter the exiting loop above
				s.pq.Push(exiting)
				// cancel all currently executing jobs
				s.el.RLock()
				defer s.el.RUnlock()
				for _, te := range s.executing {
					_ = te.Stop(false)
				}
				s.log.Infof("stopped executing jobs")
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
						delete(s.executing, plan.Job.ID())
						s.el.Unlock()
						if plan.Job.Err() != nil {
							s.log.Debugf("job %s ended with err: %s", plan.Job.ID(), plan.Job.Err())
						} else {
							s.log.Debugf("job %s completed succseefully", plan.Job.ID())
						}
						<-s.workers
					}(plan)
					s.log.Debugf("plan of job %s received", plan.Job.ID())
					te := executor.New(plan.Job, plan.Opts...)
					s.el.Lock()
					s.executing[plan.Key()] = te
					s.el.Unlock()
					_ = te.Start(plan.Ctx)
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
	if te, ok := s.executing[id]; ok {
		return te.Stats()
	}
	return nil
}
