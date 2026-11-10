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

var _ Manager = (*manager)(nil)

type manager struct {
	log log.Logger

	name string

	cm *cron.Cron
	pq staque.Priority[*registration]

	concurrent int
	el         *sync.RWMutex   // lock for registered and running
	ew         *sync.WaitGroup // wait group for executing
	registered map[string]*registration
	running    map[*Task]*execution
	arrivals   uint64 // numbers each entry into the queue, for first come first served

	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func New(opts ...Option) Manager {
	return newScheduler(opts...)
}

func newScheduler(opts ...Option) *manager {
	m := &manager{
		el:         &sync.RWMutex{},
		ew:         &sync.WaitGroup{},
		registered: make(map[string]*registration),
		running:    make(map[*Task]*execution),
		wg:         &sync.WaitGroup{},
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.pq = staque.NewPriority(
		staque.WithLessFunc(queueOrder),
		staque.WithLogger[*registration](m.log),
		staque.BlockIfEmpty[*registration](),
	)
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
		if t.ID() == "" {
			continue
		}
		if err := m.add(t); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) add(t *Task) error {
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
	if t.isOneShot() {
		m.enqueue(reg)
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
		m.el.Lock()
		if reg, ok := m.registered[key]; ok && reg.task == t {
			delete(m.registered, key)
			m.drop(reg)
		}
		// admission happens under el too, so an execution is either here or never starts
		e, running := m.running[t]
		if running {
			going = append(going, e.done)
		}
		m.el.Unlock()
		t.Job.Cancel()
		if running {
			e.cancel()
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
	// runs share the queue and the registrations, so one stopped without waiting ends first
	m.wg.Wait()
	ctx, m.cancel = context.WithCancel(ctx)
	pipe := make(chan *registration)
	workers := make(chan struct{}, m.concurrent)
	fetched := make(chan struct{})
	m.cm.Start()
	m.wg.Add(2)
	go func() {
		defer m.wg.Done()
		defer close(fetched)
		m.fetch(ctx, pipe)
	}()
	go func() {
		defer m.wg.Done()
		m.dispatch(ctx, pipe, workers, fetched)
	}()
	return nil
}

func (m *manager) fetch(ctx context.Context, pipe chan<- *registration) {
	defer close(pipe) // its only sender, so this cannot race a send
	for {
		reg, _ := m.pq.Pop()
		if ctx.Err() != nil {
			m.log.Infof("stopped fetching execution tasks")
			return
		}
		if !reg.valid() {
			continue
		}
		if reg.exclusive() {
			m.log.Debugf("task %s wait for all other tasks to complete...", reg.Key())
			m.ew.Wait()
			m.log.Debugf("continue on current exclusive task %s...", reg.Key())
		}
		// before the handoff: a quick worker may call Done as soon as it receives
		m.ew.Add(1)
		select {
		case <-ctx.Done():
			m.ew.Done()
			m.log.Infof("stopped fetching execution tasks")
			return
		case pipe <- reg:
		}
		if reg.exclusive() {
			// block all other tasks from being popped
			m.log.Debugf("wait for current exclusive task %s to complete...", reg.Key())
			m.ew.Wait()
			m.log.Debug("continue popping other tasks...")
		}
	}
}

func (m *manager) dispatch(ctx context.Context, pipe <-chan *registration, workers chan struct{}, fetched <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			wake := &registration{}
			m.pq.Push(wake)
			m.halt()
			<-fetched
			m.pq.Remove(wake) // left if fetch ended at the handoff instead of taking it
			m.log.Infof("stopped executing tasks")
			return
		case workers <- struct{}{}:
			go m.work(ctx, pipe, workers)
		}
	}
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

func (m *manager) work(runCtx context.Context, pipe <-chan *registration, workers <-chan struct{}) {
	defer func() {
		<-workers
	}()
	popped, ok := <-pipe
	if !ok {
		return
	}
	task := popped.task
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
	// queued too if its task was removed and added again while this was handed over
	m.pq.Remove(reg)
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
	m.running[task] = e
	m.el.Unlock()
	var err error
	defer func() {
		m.settle(runCtx, execCtx, e, err)
		cancel()
		m.ew.Done() // unblock task queue before releasing the worker
	}()
	err = reg.exec.Start(execCtx, params)
	if err != nil {
		m.log.Debugf("task %s ended with err: %s", task.ID(), err)
	} else {
		m.log.Debugf("task %s completed successfully", task.ID())
	}
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
	e, running := m.running[task]
	if !running && !reg.queued && reg.cooldown == nil {
		m.el.Unlock()
		return errors.Conflict.Newf("task %s is not running", id)
	}
	if reg.queued || reg.cooldown != nil {
		m.stopWaiting(reg)
		if !running && task.isOneShot() {
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

func (m *manager) Stats(id string) *Stats {
	m.el.RLock()
	defer m.el.RUnlock()
	reg, ok := m.registered[id]
	if !ok {
		return nil
	}
	st := &Stats{Stats: *reg.exec.Stats(), Requeues: reg.requeues, Requeueing: reg.requeueing()}
	if left := time.Until(reg.cooling); left > 0 {
		st.Cooldown = left
	}
	return st
}

// registration lasts from Add to Remove; its executor outlives a single execution so that retry
// state carries over from one push to the next.
type registration struct {
	task *Task
	exec executor.Executor

	// guarded by the manager's el
	cron     cron.EntryID // its schedule's entry, 0 without one
	queued   bool         // in the queue, or held on its way to a worker
	params   any          // what Trigger gave this firing, in place of Task.Params
	requeues uint         // times the current firing went back to the queue
	cooldown *time.Timer  // set while a failed execution cools down
	cooling  time.Time    // when the cooldown ends
	unwatch  func() bool  // set while a rerun waits, which Task.Ctx ending stops
	priority int          // the task's Priority, as the queue orders it
	seq      uint64       // its latest arrival in the queue
}

// requeueing: a failed execution waits to run again, cooling down or back in the queue.
func (r *registration) requeueing() bool {
	return r.cooldown != nil || (r.queued && r.requeues > 0)
}

// execution lasts from admission until it settles, which can be after Remove has forgotten its
// registration.
type execution struct {
	reg    *registration
	cancel context.CancelFunc // ends it, its retry waits included
	done   chan struct{}      // closed as it settles
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

func newExecutor(t *Task) executor.Executor {
	return executor.New(t.Job,
		executor.WithTimeout(t.Timeout),
		executor.WithRetry(t.RetryAttempts, t.RetryDelay),
	)
}
