package tasks

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"xhanio/framingo/pkg/types/common"
	"xhanio/framingo/pkg/util/task"
	"xhanio/framingo/pkg/util/task/executor"
)

type Manager interface {
	common.Service
	common.Debuggable
	common.Daemon
	Create(id string, fn task.Func, opts ...task.Option) (task.Task, error)
	Execute(ctx context.Context, t task.Task, schedule string, prioriry int, opts ...executor.Option) error
	Cancel(id string) error
	Delete(id string, force bool) error
	GetResult(id string) (any, error)
	Stats(opts StatsOptions) ([]*Stats, error)
}

type StatsOptions struct {
	Selector string `json:"selector,omitempty"`
	All      bool   `json:"all"`
}

type Stats struct {
	ID            string        `json:"id"`
	Schedule      string        `json:"schedule"`
	State         string        `json:"state"`
	Progress      float64       `json:"progress"`
	StartedAt     time.Time     `json:"started_at"`
	ExecutionTime time.Duration `json:"execution_time"`
	Cooldown      time.Duration `json:"cooldown"`
	Retries       uint          `json:"retries"`
	Labels        labels.Set    `json:"labels"`
	Error         string        `json:"error"`
}
