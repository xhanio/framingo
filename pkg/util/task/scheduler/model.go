package scheduler

import (
	"context"

	"xhanio/framingo/pkg/types/common"
	"xhanio/framingo/pkg/util/task"
	"xhanio/framingo/pkg/util/task/executor"
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
	Task      task.Task
	Ctx       context.Context
	Schedule  string
	Priority  int
	Exclusive bool
	Opts      []executor.Option
}

func (p *Plan) Key() string {
	if p == nil || p.Task == nil {
		return ""
	}
	return p.Task.ID()
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
	return p != nil && p.Task != nil
}

var statePriority = map[task.State]int{
	task.StateCreated:   0,
	task.StateRunning:   1,
	task.StateCanceling: 1,
	task.StateCanceled:  2,
	task.StateFailed:    2,
	task.StateSucceeded: 2,
}

func priorityFunc(a, b *Plan) bool {
	if a == nil || a.Task == nil {
		return true
	}
	if b == nil || b.Task == nil {
		return false
	}
	stateDiff := statePriority[a.Task.State()] - statePriority[b.Task.State()]
	if stateDiff != 0 {
		return stateDiff < 0
	}
	priorityDiff := a.Priority - b.Priority
	if priorityDiff != 0 {
		return priorityDiff < 0
	}
	timeDiff := a.Task.CreatedAt().Sub(b.Task.CreatedAt())
	if timeDiff != 0 {
		return timeDiff < 0
	}
	return a.Key() < b.Key()
}
