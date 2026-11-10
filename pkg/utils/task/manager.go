package task

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

var _ Manager = (*manager)(nil)

type manager struct {
	log log.Logger

	name string

	cm   *cron.Cron
	pq   staque.Priority[*registration]
	wake *registration // what Stop pushes for the loop waiting on an empty queue

	concurrent int
	el         *sync.RWMutex // lock for registered and running
	registered map[string]*registration
	running    map[*Task]*execution
	arrivals   uint64             // numbers each entry into the queue, for first come first served
	executions syncutil.WaitGroup // whichever Start they came from

	cancel context.CancelFunc
	looped chan struct{} // closed as the loop of the latest Start returns
}

func New(opts ...Option) Manager {
	return newScheduler(opts...)
}

func newScheduler(opts ...Option) *manager {
	m := &manager{
		wake:       &registration{},
		el:         &sync.RWMutex{},
		registered: make(map[string]*registration),
		running:    make(map[*Task]*execution),
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
		m.loop(ctx, make(chan struct{}, m.concurrent))
	}()
	return nil
}

// loop runs the queue in slots of its own Start, so that what a forced stop leaves running
// takes none from the next.
func (m *manager) loop(ctx context.Context, slots chan struct{}) {
	defer m.log.Infof("stopped executing tasks")
	for take(ctx, slots, 1) {
		reg := m.pop(ctx)
		if reg == nil {
			return
		}
		held := 1
		if reg.exclusive() {
			// every slot, so that it runs alone
			if !take(ctx, slots, m.concurrent-1) {
				return
			}
			held = m.concurrent
		}
		run := m.admit(ctx, reg)
		if run == nil {
			give(slots, held)
			continue
		}
		m.executions.Go(func() {
			defer give(slots, held)
			run()
		})
	}
}

func take(ctx context.Context, slots chan<- struct{}, n int) bool {
	for range n {
		select {
		case <-ctx.Done():
			return false
		case slots <- struct{}{}:
		}
	}
	return true
}

func give(slots <-chan struct{}, n int) {
	for range n {
		<-slots
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
	if e, busy := m.running[task]; busy {
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
	m.running[task] = e
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
	defer m.el.RUnlock()
	for _, e := range m.running {
		e.reg.task.Job.Signal(job.SignalKill)
	}
	return len(m.running)
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
		if task.isOneShot() {
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
	queued   bool         // waiting to run: in the queue, popped, or held back by its task's last execution
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

// execution lasts from admission until it settles, which can be after a Remove or a Stop has
// forgotten its registration.
type execution struct {
	reg    *registration
	cancel context.CancelFunc // ends it, its retry waits included
	done   chan struct{}      // closed as it settles
	next   *registration      // queued for its task meanwhile, to queue again as it settles
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
