package job

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
)

var _ Job = (*job)(nil)

type job struct {
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

func New(id string, fn Func, opts ...Option) Job {
	return newJob(id, fn, opts...)
}

func newJob(id string, fn Func, opts ...Option) *job {
	if id == "" {
		id = uuid.NewString()
	}
	j := &job{
		id:        id,
		fn:        fn,
		labels:    make(labels.Set),
		state:     StateCreated,
		createdAt: time.Now(),
		wg:        &sync.WaitGroup{},
		progress:  -1,
	}
	for _, opt := range opts {
		opt(j)
	}
	if j.log == nil {
		j.log = log.Default
	}
	j.log = j.log.With(zap.String("job", id))
	return j
}

func (j *job) ID() string {
	return j.id
}

func (j *job) Key() string {
	return j.id
}

func (j *job) initialize() {
	j.state = StateRunning
	j.startedAt = time.Now()
	j.endedAt = time.Time{}
	j.result = nil
	// j.sendEvent(JobActionUpdate)
}

func (j *job) Start(ctx context.Context) bool {
	j.RLock()
	// return if job is running, or is done without running once or without cooldown.
	if IsPending(j.state) {
		j.log.Debugf("job %s is still running", j.id)
		j.RUnlock()
		return false
	}
	j.RUnlock()

	// run job
	j.wg.Add(1)
	go func() {
		// finalize
		defer func() {
			j.Lock()
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					j.err = e
				} else {
					j.err = errors.Newf("job %s recovered from panic: %v", j.id, r)
				}
			}
			j.endedAt = time.Now()
			if j.state == StateCanceling {
				j.state = StateCanceled
			} else if j.err != nil {
				// j.log.Error(j.err)
				j.state = StateFailed
			} else {
				j.state = StateSucceeded
			}
			// j.sendEvent(JobActionUpdate)
			j.Unlock()
			// unblock job
			j.wg.Done()
		}()
		j.Lock()
		// initialize
		j.initialize()
		// set params
		params := ctx.Value(ContextKeyParams)
		if p, ok := params.([]any); ok {
			j.params = p
		}
		j.Unlock()

		j.ctx, j.cancel = context.WithCancel(ctx)
		j.err = j.fn(j)
	}()
	return true
}

func (j *job) Wait() {
	j.wg.Wait()
}

func (j *job) Cancel() bool {
	if j.State() == StateRunning && j.cancel != nil {
		j.log.Debugf("canceling job %s", j.id)
		j.Lock()
		j.state = StateCanceling
		// j.sendEvent(JobActionUpdate)
		j.Unlock()
		j.cancel()
		j.cancel = nil
		return true
	}
	return false
}

func (j *job) Context() context.Context {
	if j.ctx == nil {
		return context.Background()
	}
	return j.ctx
}

func (j *job) Result() any {
	j.RLock()
	defer j.RUnlock()
	return j.result
}

func (j *job) Err() error {
	j.RLock()
	defer j.RUnlock()
	return j.err
}

func (j *job) State() State {
	j.RLock()
	defer j.RUnlock()
	return j.state
}

func (j *job) Logger() log.Logger {
	return j.log
}

func (j *job) CreatedAt() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.createdAt
}

func (j *job) StartedAt() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.startedAt
}

func (j *job) EndedAt() time.Time {
	j.RLock()
	defer j.RUnlock()
	return j.endedAt
}

func (j *job) ExecutionTime() time.Duration {
	j.RLock()
	defer j.RUnlock()
	if IsPending(j.state) {
		return time.Since(j.startedAt)
	}
	return j.endedAt.Sub(j.startedAt)
}

func (j *job) Progress() float64 {
	j.RLock()
	defer j.RUnlock()
	return j.progress
}

func (j *job) Labels() labels.Set {
	j.RLock()
	defer j.RUnlock()
	return j.labels
}

func (j *job) IsDone() bool {
	j.RLock()
	defer j.RUnlock()
	return IsDone(j.state)
}

func (j *job) IsState(state State) bool {
	j.RLock()
	defer j.RUnlock()
	return j.state == state
}

func (j *job) IsExecuting() bool {
	j.RLock()
	defer j.RUnlock()
	return IsPending(j.state)
}

func (j *job) SetProgress(progress float64) {
	j.progress = progress
	// j.sendEvent(JobActionUpdate)
}

func (j *job) SetResult(result any) {
	j.result = result
}

func (j *job) GetParams() []any {
	return j.params
}

// func (j *job) sendEvent(action JobAction) {
// 	if j.onEvent != nil {
// 		j.onEvent(Event{ID: j.id, Action: action, State: j.state, Progress: j.progress})
// 	}
// }

func (j *job) stats() *Stats {
	stats := &Stats{
		ID:        j.id,
		State:     string(j.state),
		Progress:  j.progress,
		StartedAt: j.startedAt,
		Labels:    j.labels,
	}
	if IsPending(j.state) {
		stats.ExecutionTime = time.Since(j.startedAt)
	} else {
		stats.ExecutionTime = j.endedAt.Sub(j.startedAt)
	}
	if j.err != nil {
		stats.Error = j.err.Error()
	}
	return stats
}

func (j *job) Stats() *Stats {
	j.RLock()
	defer j.RUnlock()
	return j.stats()
}
