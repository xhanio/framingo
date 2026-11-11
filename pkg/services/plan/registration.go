package plan

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/entity"
)

// registration lasts from Add to Delete: the plan, and the latest run of a plan of a kind, which
// its runs write as they start and end. A run left going by a Delete writes to its own
// registration, never to that of the plan added again under the same ID.
type registration struct {
	plan *entity.Plan

	mu     sync.Mutex
	latest *entity.PlanRun
}

// lastRun is the plan's latest run, nil if it has none yet or the plan has no kind.
func (r *registration) lastRun() *entity.PlanRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	run := *r.latest
	return &run
}

func (r *registration) record(run entity.PlanRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latest = &run
}
