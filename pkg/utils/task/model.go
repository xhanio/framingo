package task

import (
	"context"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

const DefaultGracePeriod = 10 * time.Second

type Manager interface {
	common.Service
	common.Daemon
	Add(tasks ...*Task) error
	// Remove forgets the tasks and ends what they are doing, then waits for their executions
	// until ctx ends; nothing of them starts after Remove. What still runs once ctx ends is
	// sent SignalKill, like Stop, and the error wraps ctx's.
	Remove(ctx context.Context, tasks ...*Task) error
	Trigger(id string, params any) error
	Cancel(id string) error
	Stats(id string) *Stats
}

// Stats is what the task's executor reports, plus what the manager adds on top of it.
type Stats struct {
	executor.Stats
	Requeues uint          `json:"requeues"` // times the current firing went back to the queue
	Cooldown time.Duration `json:"cooldown"` // what is left of the current RequeueCooldown
	// Requeueing says the latest execution failed and the task waits to run again, cooling
	// down or back in the queue. Its job stays failed meanwhile: that run did fail.
	Requeueing bool `json:"requeueing"`
}

// Task is configuration; the manager keeps its state of a task elsewhere, so a Task copies like
// any value.
type Task struct {
	Job      job.Job         `json:"-"`
	Ctx      context.Context `json:"-"`
	Params   any             `json:"params"`
	Schedule string          `json:"schedule"`
	// Manual keeps the task registered without running it on its own: only Trigger does.
	Manual   bool `json:"manual,omitempty"`
	Priority int  `json:"priority"` // lower runs first; within one, in the order queued
	// Exclusive runs it alone: it takes every slot, so it waits for the executions going on
	// and nothing else starts until it ends. What a forced stop left running holds no slot.
	Exclusive     bool          `json:"exclusive"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	RetryAttempts int           `json:"retry_attempts,omitempty"`
	RetryDelay    time.Duration `json:"retry_delay,omitempty"`
	// GracePeriod is how long a canceled or timed out execution gets to end on its own, for
	// whoever stops it to honor; the manager itself never reads it. Zero means
	// DefaultGracePeriod. A Command has its own Grace for how it stops.
	GracePeriod time.Duration `json:"grace_period,omitempty"`
	// RequeueAttempts is how many times a failed execution, its retries used
	// up, goes back to the queue. Unlike a retry it gives up its slot while
	// it waits, and it counts on top: a task runs up to
	// (RetryAttempts+1) * (RequeueAttempts+1) times.
	RequeueAttempts int `json:"requeue_attempts,omitempty"`
	// RequeueCooldown is how long a failed execution waits before going back to
	// the queue.
	RequeueCooldown time.Duration `json:"requeue_cooldown,omitempty"`
}

// ID is the task's identity, its job's ID; look tasks up by ID only.
func (t *Task) ID() string {
	if t == nil || t.Job == nil {
		return ""
	}
	return t.Job.ID()
}

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

func (t *Task) String() string {
	return t.ID()
}

func (t *Task) IsValid() bool {
	return t != nil && t.Job != nil
}
