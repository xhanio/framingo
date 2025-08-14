package task

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/structs/job"
	"github.com/xhanio/framingo/pkg/structs/job/executor"
	"github.com/xhanio/framingo/pkg/structs/job/scheduler"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

const timeFormat = "2006-01-02 15:04:05.00"

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	es common.EventSender

	defaults []job.Option

	s scheduler.Scheduler
	sync.RWMutex
	plans map[string]*scheduler.Plan
}

func New(es common.EventSender, opts ...Option) Manager {
	return newManager(es, opts...)
}

func newManager(es common.EventSender, opts ...Option) *manager {
	m := &manager{
		es:    es,
		plans: make(map[string]*scheduler.Plan),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	m.s = scheduler.New(
		scheduler.MaxConcurrency(10),
		scheduler.WithLogger(m.log),
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

func (m *manager) Create(id string, fn job.Func, opts ...job.Option) (job.Job, error) {
	if fn == nil {
		return nil, errors.Newf("failed to create job: job func undefined")
	}
	var options []job.Option
	// apply manager options
	options = append(options, m.defaults...)
	// apply job options
	options = append(options, opts...)
	t := job.New(id, fn, options...)
	return t, nil
}

func (m *manager) Execute(ctx context.Context, t job.Job, schedule string, prioriry int, opts ...executor.Option) error {
	m.Lock()
	defer m.Unlock()
	if t == nil {
		return nil
	}
	plan := &scheduler.Plan{
		Job:      t,
		Ctx:      ctx,
		Schedule: schedule,
		Priority: prioriry,
		Opts:     opts,
	}
	err := m.s.Add(plan)
	if err != nil {
		return errors.Wrap(err)
	}
	m.plans[t.ID()] = plan
	return nil
}

func (m *manager) Cancel(id string) error {
	m.RLock()
	defer m.RUnlock()
	if plan, ok := m.plans[id]; ok {
		plan.Job.Cancel()
		return nil
	}
	return errors.NotFound.Newf("failed to cancel job %s: job id not found", id)
}

func (m *manager) Delete(id string, force bool) error {
	m.Lock()
	defer m.Unlock()
	plan, ok := m.plans[id]
	if ok {
		m.s.Remove(plan)
		if force {
			plan.Job.Cancel()
		}
		delete(m.plans, id)
		return nil
	}
	return errors.NotFound.Newf("job id %s not found", id)
}

func (m *manager) GetResult(id string) (any, error) {
	m.RLock()
	defer m.RUnlock()
	if plan, ok := m.plans[id]; ok {
		return plan.Job.Result(), nil
	}
	return nil, errors.NotFound.Newf("failed to get result of job %s: job id not found", id)
}

func (m *manager) stats(all bool) []*Stats {
	result := make([]*Stats, 0)
	for _, plan := range m.plans {
		t := plan.Job
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
		stats.Schedule = plan.Schedule
		ss := m.s.Stats(t.ID())
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
	for _, job := range m.stats(opts.All) {
		if selector.Empty() || selector.Matches(job.Labels) {
			result = append(result, job)
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
	err := m.s.Start(ctx)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	var errs []error
	errs = append(errs, m.s.Stop(wait))
	return errors.Combine(errs...)
}
