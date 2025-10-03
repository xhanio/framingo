package job

import (
	"context"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/log"

	"k8s.io/apimachinery/pkg/labels"
)

type Func func(ctx Context) error

func Wrap(fn func(context.Context) error) Func {
	return func(jc Context) error {
		return errors.Wrap(fn(jc.Context()))
	}
}

type Context interface {
	ID() string
	Context() context.Context
	Logger() log.Logger
	Labels() labels.Set
	SetProgress(progress float64)
	SetResult(result any)
	GetParams() any
}

type Job interface {
	ID() string
	Labels() labels.Set
	CreatedAt() time.Time
	StartedAt() time.Time
	EndedAt() time.Time
	Run(ctx context.Context, params any) bool
	Wait()
	Cancel() bool
	Result() any
	Err() error
	State() State
	Context() context.Context
	Progress() float64
	ExecutionTime() time.Duration
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
