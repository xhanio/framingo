package entity

import (
	"context"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

// Task is configuration only and copies like any value.
type Task struct {
	Job      job.Job         `json:"-"`
	Ctx      context.Context `json:"-"`
	Params   any             `json:"params"`
	Schedule string          `json:"schedule"`
	// Manual tasks run only via Trigger.
	Manual   bool `json:"manual,omitempty"`
	Priority int  `json:"priority"` // lower runs first, then FIFO
	// Exclusive takes every slot; leftovers of a forced stop hold none.
	Exclusive     bool          `json:"exclusive"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	RetryAttempts int           `json:"retry_attempts,omitempty"`
	RetryDelay    time.Duration `json:"retry_delay,omitempty"`
	// a requeue frees its slot while waiting; a task runs up to
	// (RetryAttempts+1) * (RequeueAttempts+1) times
	RequeueAttempts int           `json:"requeue_attempts,omitempty"`
	RequeueCooldown time.Duration `json:"requeue_cooldown,omitempty"`
}

// ID is the job's ID; look tasks up by ID only.
func (t *Task) ID() string {
	if t == nil || t.Job == nil {
		return ""
	}
	return t.Job.ID()
}

func (t *Task) String() string {
	return t.ID()
}

// IsOneShot tasks leave the task manager after they run.
func (t *Task) IsOneShot() bool {
	return t.Schedule == "" && !t.Manual
}

func (t *Task) IsValid() bool {
	return t != nil && t.Job != nil
}

type TaskStats struct {
	executor.Stats
	Requeues uint          `json:"requeues"` // of the current firing
	Cooldown time.Duration `json:"cooldown"` // remaining RequeueCooldown
	// while Requeueing the job stays failed: that run did fail
	Requeueing bool `json:"requeueing"`
}
