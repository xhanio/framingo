package scheduler

import (
	"context"

	"xhanio/framingo/pkg/structs/job"
	"xhanio/framingo/pkg/structs/job/executor"
	"xhanio/framingo/pkg/types/common"
)

var (
	_ common.Unique   = (*Plan)(nil)
	_ common.Weighted = (*Plan)(nil)
)

type Scheduler interface {
	common.Service
	common.Daemon
	Add(plans ...*Plan) error
	Remove(plans ...*Plan)
	Stats(id string) *executor.Stats
}

type Plan struct {
	Job       job.Job
	Ctx       context.Context
	Schedule  string
	Priority  int
	Exclusive bool
	Opts      []executor.Option
}

func (p *Plan) Key() string {
	if p == nil || p.Job == nil {
		return ""
	}
	return p.Job.ID()
}

func (p *Plan) GetPriority() int {
	if p == nil {
		return 0
	}
	return p.Priority
}

func (p *Plan) SetPriority(priority int) {
	if p == nil {
		return
	}
	p.Priority = priority
}

func (p *Plan) String() string {
	return p.Key()
}

func (p *Plan) IsValid() bool {
	return p != nil && p.Job != nil
}

var statePriority = map[job.State]int{
	job.StateCreated:   0,
	job.StateRunning:   1,
	job.StateCanceling: 1,
	job.StateCanceled:  2,
	job.StateFailed:    2,
	job.StateSucceeded: 2,
}

func priorityFunc(a, b *Plan) bool {
	if a == nil || a.Job == nil {
		return true
	}
	if b == nil || b.Job == nil {
		return false
	}
	stateDiff := statePriority[a.Job.State()] - statePriority[b.Job.State()]
	if stateDiff != 0 {
		return stateDiff < 0
	}
	priorityDiff := a.Priority - b.Priority
	if priorityDiff != 0 {
		return priorityDiff < 0
	}
	timeDiff := a.Job.CreatedAt().Sub(b.Job.CreatedAt())
	if timeDiff != 0 {
		return timeDiff < 0
	}
	return a.Key() < b.Key()
}
