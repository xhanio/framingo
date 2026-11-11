package model

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Plan interface {
	common.Service
	// Add keeps a copy of the plan, which needs an ID, a kind, and a Task with a Schedule or
	// Manual, since a plan stays until Delete. The plan manager builds the task's Job from the
	// kind, under the plan's ID; an ID already taken on the task manager is a Conflict.
	Add(plan *entity.Plan) error
	// Trigger runs the plan now and returns the run's ID.
	// source goes on the run and on its retries and requeues; empty is an InvalidArgument.
	// The run always ends with PlanRunFinished: canceled, without a PlanRunStarted, if a
	// Cancel, Delete or Stop drops it before it starts.
	Trigger(id string, source entity.PlanRunSource) (string, error)
	// Cancel ends what the plan is doing, a run Trigger queued included, and keeps the plan.
	Cancel(id string) error
	// Delete ends what the plan is doing, forgets the plan, waits for its run to end until ctx
	// ends, and returns its latest run, nil if it has none. Its cron entry goes first, so no
	// new run starts meanwhile. Once ctx ends, what still runs is sent SignalKill, and the
	// error wraps ctx's: the run returned may then still be running, and its end comes as
	// PlanRunFinished. An ended ctx makes it a forced delete.
	Delete(ctx context.Context, id string) (*entity.PlanRun, error)
	// LastRun returns the plan's latest run, nil if it has none yet.
	LastRun(id string) (*entity.PlanRun, error)
	// OpenRunLog opens the log file of a run, by its PlanRun.LogPath, which may still be
	// being written. With tailBytes > 0 it reads from that many bytes before the end. A path
	// outside the log dir is Forbidden; no log dir or no such file is NotFound.
	OpenRunLog(path string, tailBytes int64) (io.ReadCloser, error)
	Stats(opts entity.PlanStatsOptions) ([]*entity.PlanStats, error)
}
