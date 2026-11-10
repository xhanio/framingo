package model

import (
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Planner interface {
	common.Service
	Add(plan *entity.Plan) error
	// Trigger runs the plan now and returns the run's ID, empty for a plan without a kind.
	// source goes on the run and on its retries and requeues; empty is an InvalidArgument.
	Trigger(id string, source entity.PlanRunSource) (string, error)
	Cancel(id string) error
	// Delete ends what the plan is doing, forgets the plan and returns its latest run, nil if
	// it has none or has no kind. Its cron entry goes first, so no new run starts meanwhile.
	// A plan's job gets the GracePeriod of its task to end, and SignalKill once it is up;
	// force sends SignalKill at once, and does not wait for a plan without a kind, whose
	// goroutine Go cannot kill.
	Delete(id string, force bool) (*entity.PlanRun, error)
	// LastRun returns the plan's latest run, nil if it has none yet or has no kind.
	LastRun(id string) (*entity.PlanRun, error)
	// OpenRunLog opens the log file of a run, by its PlanRun.LogPath, which may still be
	// being written. With tailBytes > 0 it reads from that many bytes before the end. A path
	// outside the log dir is Forbidden; no log dir or no such file is NotFound.
	OpenRunLog(path string, tailBytes int64) (io.ReadCloser, error)
	// GetResult returns the result of a plan's own job.
	GetResult(id string) (any, error)
	Stats(opts entity.PlannerStatsOptions) ([]*entity.PlannerStats, error)
}
