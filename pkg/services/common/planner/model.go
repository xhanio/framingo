package planner

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/task"
)

type Manager interface {
	common.Service
	common.Debuggable
	common.Daemon
	Create(id string, fn job.Func, opts ...job.Option) (job.Job, error)
	Add(plan *task.Task) error
	Cancel(id string) error
	Delete(id string, force bool) error
	GetResult(id string) (any, error)
	Stats(opts StatsOptions) ([]*Stats, error)
}

type TODO struct {
	ID          string
	Description string
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
