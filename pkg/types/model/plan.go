package model

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Plan interface {
	common.Service
	// Add copies the plan, which needs an ID, a kind and a Task with Schedule or
	// Manual; an ID taken on the task manager is a Conflict.
	Add(plan *entity.Plan) error
	// Trigger returns the run ID; empty source is InvalidArgument. The run always
	// ends with PlanRunFinished, without PlanRunStarted if dropped before starting.
	Trigger(id string, source entity.PlanRunSource) (string, error)
	// Cancel also drops a queued Trigger run, and keeps the plan.
	Cancel(id string) error
	// Delete waits for the run until ctx ends, then sends SignalKill and returns
	// an error wrapping ctx's; the returned run (nil if none) may then still be
	// running, its end reported as PlanRunFinished.
	Delete(ctx context.Context, id string) (*entity.PlanRun, error)
	// LastRun is nil if the plan has not run.
	LastRun(id string) (*entity.PlanRun, error)
	// OpenRunLog's file may still be written; tailBytes > 0 starts that far from
	// the end. A path outside the log dir is Forbidden, a missing one NotFound.
	OpenRunLog(path string, tailBytes int64) (io.ReadCloser, error)
	Stats(opts entity.PlanStatsOptions) ([]*entity.PlanStats, error)
}
