package plan

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/entity"
)

// registration lasts from Add to Delete; a run outliving a Delete writes to its own
// registration, never to one re-added under the same ID.
type registration struct {
	plan *entity.Plan

	mu      sync.Mutex
	latest  *entity.PlanRun
	pending *triggered // the latest firing of Trigger, whose ID a run or a drop claims
}

func (r *registration) lastRun() *entity.PlanRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil
	}
	run := *r.latest
	return &run
}

func (r *registration) setPending(tr *triggered) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = tr
}

// unstarted claims the latest firing of Trigger, if no run of it has.
func (r *registration) unstarted() *triggered {
	r.mu.Lock()
	defer r.mu.Unlock()
	tr := r.pending
	r.pending = nil
	if tr == nil || !tr.claim() {
		return nil
	}
	return tr
}

func (r *registration) record(run entity.PlanRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latest = &run
}
