package task

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"

	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/log"
)

var _ Task = (*task)(nil)

type task struct {
	id     string
	labels labels.Set
	fn     Func

	log log.Logger

	sync.RWMutex // state lock
	state        State
	params       []any // input
	result       any   // output
	err          error
	createdAt    time.Time
	startedAt    time.Time
	endedAt      time.Time

	progress float64

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func New(id string, fn Func, opts ...Option) Task {
	return newTask(id, fn, opts...)
}

func newTask(id string, fn Func, opts ...Option) *task {
	if id == "" {
		id = uuid.NewString()
	}
	t := &task{
		id:        id,
		fn:        fn,
		labels:    make(labels.Set),
		state:     StateCreated,
		createdAt: time.Now(),
		wg:        &sync.WaitGroup{},
		progress:  -1,
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.log == nil {
		t.log = log.Default
	}
	t.log = t.log.With(zap.String("task", id))
	return t
}

func (t *task) ID() string {
	return t.id
}

func (t *task) Key() string {
	return t.id
}

func (t *task) initialize() {
	t.state = StateRunning
	t.startedAt = time.Now()
	t.endedAt = time.Time{}
	t.result = nil
	// t.sendEvent(TaskActionUpdate)
}

func (t *task) Start(ctx context.Context) bool {
	t.RLock()
	// return if task is running, or is done without running once or without cooldown.
	if IsPending(t.state) {
		t.log.Debugf("task %s is still running", t.id)
		t.RUnlock()
		return false
	}
	t.RUnlock()

	// run task
	t.wg.Add(1)
	go func() {
		// finalize
		defer func() {
			t.Lock()
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					t.err = e
				} else {
					t.err = errors.Newf("task %s recovered from panic: %v", t.id, r)
				}
			}
			t.endedAt = time.Now()
			if t.state == StateCanceling {
				t.state = StateCanceled
			} else if t.err != nil {
				// t.log.Error(t.err)
				t.state = StateFailed
			} else {
				t.state = StateSucceeded
			}
			// t.sendEvent(TaskActionUpdate)
			t.Unlock()
			// unblock task
			t.wg.Done()
		}()
		t.Lock()
		// initialize
		t.initialize()
		// set params
		params := ctx.Value(ContextKeyParams)
		if p, ok := params.([]any); ok {
			t.params = p
		}
		t.Unlock()

		t.ctx, t.cancel = context.WithCancel(ctx)
		t.err = t.fn(t)
	}()
	return true
}

func (t *task) Wait() {
	t.wg.Wait()
}

func (t *task) Cancel() bool {
	if t.State() == StateRunning && t.cancel != nil {
		t.log.Debugf("canceling task %s", t.id)
		t.Lock()
		t.state = StateCanceling
		// t.sendEvent(TaskActionUpdate)
		t.Unlock()
		t.cancel()
		t.cancel = nil
		return true
	}
	return false
}

func (t *task) Context() context.Context {
	if t.ctx == nil {
		return context.Background()
	}
	return t.ctx
}

func (t *task) Result() any {
	t.RLock()
	defer t.RUnlock()
	return t.result
}

func (t *task) Err() error {
	t.RLock()
	defer t.RUnlock()
	return t.err
}

func (t *task) State() State {
	t.RLock()
	defer t.RUnlock()
	return t.state
}

func (t *task) Logger() log.Logger {
	return t.log
}

func (t *task) CreatedAt() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.createdAt
}

func (t *task) StartedAt() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.startedAt
}

func (t *task) EndedAt() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.endedAt
}

func (t *task) ExecutionTime() time.Duration {
	t.RLock()
	defer t.RUnlock()
	if IsPending(t.state) {
		return time.Since(t.startedAt)
	}
	return t.endedAt.Sub(t.startedAt)
}

func (t *task) Progress() float64 {
	t.RLock()
	defer t.RUnlock()
	return t.progress
}

func (t *task) Labels() labels.Set {
	t.RLock()
	defer t.RUnlock()
	return t.labels
}

func (t *task) IsDone() bool {
	t.RLock()
	defer t.RUnlock()
	return IsDone(t.state)
}

func (t *task) IsState(state State) bool {
	t.RLock()
	defer t.RUnlock()
	return t.state == state
}

func (t *task) IsExecuting() bool {
	t.RLock()
	defer t.RUnlock()
	return IsPending(t.state)
}

func (t *task) SetProgress(progress float64) {
	t.progress = progress
	// t.sendEvent(TaskActionUpdate)
}

func (t *task) SetResult(result any) {
	t.result = result
}

func (t *task) GetParams() []any {
	return t.params
}

// func (t *task) sendEvent(action TaskAction) {
// 	if t.onEvent != nil {
// 		t.onEvent(Event{ID: t.id, Action: action, State: t.state, Progress: t.progress})
// 	}
// }

func (t *task) stats() *Stats {
	stats := &Stats{
		ID:        t.id,
		State:     string(t.state),
		Progress:  t.progress,
		StartedAt: t.startedAt,
		Labels:    t.labels,
	}
	if IsPending(t.state) {
		stats.ExecutionTime = time.Since(t.startedAt)
	} else {
		stats.ExecutionTime = t.endedAt.Sub(t.startedAt)
	}
	if t.err != nil {
		stats.Error = t.err.Error()
	}
	return stats
}

func (t *task) Stats() *Stats {
	t.RLock()
	defer t.RUnlock()
	return t.stats()
}
