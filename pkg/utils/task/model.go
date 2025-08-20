package task

import (
	"context"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

var (
	_ common.Unique   = (*Task)(nil)
	_ common.Weighted = (*Task)(nil)
)

type Manager interface {
	common.Service
	common.Daemon
	Add(tasks ...*Task) error
	Remove(tasks ...*Task)
	Stats(id string) *executor.Stats
}

type Task struct {
	Job           job.Job         `json:"-"`
	Ctx           context.Context `json:"-"`
	Params        any             `json:"params"`
	Schedule      string          `json:"schedule"`
	Priority      int             `json:"priority"`
	Exclusive     bool            `json:"exclusive"`
	Timeout       time.Duration   `json:"timeout,omitempty"`
	Cooldown      time.Duration   `json:"cooldown,omitempty"`
	Once          bool            `json:"once"`
	RetryAttempts int             `json:"retry_attempts,omitempty"`
	RetryDelay    time.Duration   `json:"retry_delay,omitempty"`
}

func (t *Task) Key() string {
	if t == nil || t.Job == nil {
		return ""
	}
	return t.Job.ID()
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
	return t.Key()
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

func priorityFunc(a, b *Task) bool {
	if a == nil || a.Job == nil {
		return false
	}
	if b == nil || b.Job == nil {
		return true
	}
	stateDiff := statePriority[a.Job.State()] - statePriority[b.Job.State()]
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
