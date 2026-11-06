package model

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Planner interface {
	common.Service
	Add(todo *entity.Plan) error
	// Trigger runs the plan now and returns the run's ID, empty for a plan without a kind.
	// source is recorded on the run and on its retries and requeues; PlanRunManual and
	// PlanRunSchedule are the planner's own, other values the caller's. Empty is an
	// InvalidArgument. A plan without a kind has no run to record it on.
	Trigger(id string, source entity.PlanRunSource) (string, error)
	Cancel(id string) error
	Delete(id string, force bool) error
	// LastRun returns the plan's latest run, nil if it has none yet or has no kind.
	LastRun(id string) (*entity.PlanRun, error)
	// GetResult returns the result of a plan's own job.
	GetResult(id string) (any, error)
	Stats(opts entity.PlannerStatsOptions) ([]*entity.PlannerStats, error)
}
