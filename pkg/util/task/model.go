package task

import (
	"context"
	"time"

	"xhanio/framingo/pkg/util/log"

	"k8s.io/apimachinery/pkg/labels"
)

type contextKey string

var (
	ContextKeyParams = contextKey("_task_params")
)

type Func func(ctx Context) error

type Context interface {
	ID() string
	Context() context.Context
	Logger() log.Logger
	Labels() labels.Set
	SetProgress(progress float64)
	SetResult(result any)
	GetParams() []any
}

type Task interface {
	ID() string
	Labels() labels.Set
	CreatedAt() time.Time
	StartedAt() time.Time
	EndedAt() time.Time
	State() State
	Context() context.Context
	Progress() float64
	Err() error
	Result() any
	Start(ctx context.Context) bool
	ExecutionTime() time.Duration
	Wait()
	Cancel() bool
	IsExecuting() bool
	IsDone() bool
	IsState(state State) bool
	Stats() *Stats
}

type State string

var (
	StateCreated = State("created")

	StateRunning   = State("running")
	StateCanceling = State("canceling")

	StateSucceeded = State("succeeded")
	StateFailed    = State("failed")
	StateCanceled  = State("canceled")
)

type Stats struct {
	ID            string        `json:"id"`
	State         string        `json:"state"`
	Progress      float64       `json:"progress"`
	StartedAt     time.Time     `json:"started_at"`
	ExecutionTime time.Duration `json:"execution_time"`
	Labels        labels.Set    `json:"labels"`
	Error         string        `json:"error"`
}
