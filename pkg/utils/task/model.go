package task

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

const DefaultGracePeriod = 10 * time.Second

var (
	_ common.Unique   = (*Task)(nil)
	_ common.Weighted = (*Task)(nil)
)

type Manager interface {
	common.Service
	common.Daemon
	Add(tasks ...*Task) error
	// Remove forgets the tasks and ends what they are doing. The channel it returns closes
	// once none of them has an execution still going; nothing of them starts after Remove.
	Remove(tasks ...*Task) <-chan struct{}
	Trigger(id string, params any) error
	Cancel(id string) error
	Stats(id string) *Stats
}

// Stats is what the task's executor reports, plus what the manager adds on top of it.
type Stats struct {
	executor.Stats
	Requeues uint          `json:"requeues"` // times the latest execution went back to the queue
	Cooldown time.Duration `json:"cooldown"` // what is left of the current RequeueCooldown
}

type Task struct {
	Job      job.Job         `json:"-"`
	Ctx      context.Context `json:"-"`
	Params   any             `json:"params"`
	Schedule string          `json:"schedule"`
	// Manual keeps the task registered without running it on its own: only Trigger does.
	Manual        bool          `json:"manual,omitempty"`
	Priority      int           `json:"priority"`
	Exclusive     bool          `json:"exclusive"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	RetryAttempts int           `json:"retry_attempts,omitempty"`
	RetryDelay    time.Duration `json:"retry_delay,omitempty"`
	// GracePeriod is how long a canceled or timed out execution gets to end on its own, for
	// whoever stops it to honor. Zero means DefaultGracePeriod.
	GracePeriod time.Duration `json:"grace_period,omitempty"`
	// RequeueAttempts is how many times a failed execution, its retries used
	// up, goes back to the queue. Unlike a retry it gives up its worker while
	// it waits, and it counts on top: a task runs up to
	// (RetryAttempts+1) * (RequeueAttempts+1) times.
	RequeueAttempts int `json:"requeue_attempts,omitempty"`
	// RequeueCooldown is how long a failed execution waits before going back to
	// the queue.
	RequeueCooldown time.Duration `json:"requeue_cooldown,omitempty"`

	requeueing atomic.Bool // see IsRequeueing; the queue orders by it, so it only changes out of the queue
}

// ID is the task's identity, its job's ID; look tasks up by ID only.
func (t *Task) ID() string {
	if t == nil || t.Job == nil {
		return ""
	}
	return t.Job.ID()
}

// Key only satisfies common.Unique for the priority queue; it is the ID.
func (t *Task) Key() string {
	return t.ID()
}

// IsRequeueing reports whether the task failed and is waiting to run again, cooling
// down or back in the queue. Its job stays failed meanwhile: that run did fail.
func (t *Task) IsRequeueing() bool {
	return t != nil && t.requeueing.Load()
}

// Grace is the task's GracePeriod with the default applied.
func (t *Task) Grace() time.Duration {
	if t.GracePeriod <= 0 {
		return DefaultGracePeriod
	}
	return t.GracePeriod
}

// isOneShot reports whether the task leaves the manager after its execution.
func (t *Task) isOneShot() bool {
	return t.Schedule == "" && !t.Manual
}

func (t *Task) GetPriority() int {
	if t == nil {
		return 0
	}
	return t.Priority
}

func (t *Task) SetPriority(priority int) {
	if t == nil {
		return
	}
	t.Priority = priority
}

func (t *Task) String() string {
	return t.ID()
}

func (t *Task) IsValid() bool {
	return t != nil && t.Job != nil
}

var statePriority = map[job.State]int{
	job.StateCreated:   0,
	job.StateRunning:   1,
	job.StateCanceling: 1,
	job.StateCanceled:  2,
	job.StateFailed:    2,
	job.StateSucceeded: 2,
}

// rank is the task's place by state: back in the queue, a requeued task is on the
// same terms as a new one, and only its Priority sets it apart.
func (t *Task) rank() int {
	if t.requeueing.Load() {
		return statePriority[job.StateCreated]
	}
	return statePriority[t.Job.State()]
}

func priorityFunc(a, b *Task) bool {
	if a == nil || a.Job == nil {
		return false
	}
	if b == nil || b.Job == nil {
		return true
	}
	stateDiff := a.rank() - b.rank()
	if stateDiff != 0 {
		return stateDiff > 0
	}
	priorityDiff := a.Priority - b.Priority
	if priorityDiff != 0 {
		return priorityDiff > 0
	}
	timeDiff := a.Job.CreatedAt().Sub(b.Job.CreatedAt())
	if timeDiff != 0 {
		return timeDiff > 0
	}
	return a.Key() > b.Key()
}
