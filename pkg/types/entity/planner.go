package entity

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/utils/task"
)

type PlannerTODO struct {
	ID          string     `json:"id"`
	Metadata    labels.Set `json:"metadata"`
	Description string     `json:"description"`
	Task        *task.Task `json:"task"`
}

type PlannerStatsOptions struct {
	Selector string `json:"selector,omitempty"`
	All      bool   `json:"all"`
}

type PlannerStats struct {
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
