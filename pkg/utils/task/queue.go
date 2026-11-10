package task

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
)

// The priority queue holds registrations: how a task is ordered is the manager's state.

var (
	_ common.Unique   = (*registration)(nil)
	_ common.Weighted = (*registration)(nil)
)

// exiting unblocks the fetching goroutine as the manager stops.
var exiting = &registration{}

// Key is the task's ID: the queue holds one entry per ID.
func (r *registration) Key() string {
	return r.task.ID()
}

func (r *registration) GetPriority() int {
	return r.priority
}

func (r *registration) SetPriority(priority int) {
	r.priority = priority
}

func (r *registration) valid() bool {
	return r != nil && r.task.IsValid()
}

func (r *registration) exclusive() bool {
	return r.task != nil && r.task.Exclusive
}

var statePriority = map[job.State]int{
	job.StateCreated:   0,
	job.StateRunning:   1,
	job.StateCanceling: 1,
	job.StateCanceled:  2,
	job.StateFailed:    2,
	job.StateSucceeded: 2,
}

// rank is the task's place by state: back in the queue, a requeued task is on the
// same terms as a new one, and only its priority sets it apart.
func (r *registration) rank() int {
	if r.requeueing.Load() {
		return statePriority[job.StateCreated]
	}
	return statePriority[r.task.Job.State()]
}

func priorityFunc(a, b *registration) bool {
	if !a.valid() {
		return false
	}
	if !b.valid() {
		return true
	}
	stateDiff := a.rank() - b.rank()
	if stateDiff != 0 {
		return stateDiff > 0
	}
	priorityDiff := a.priority - b.priority
	if priorityDiff != 0 {
		return priorityDiff > 0
	}
	timeDiff := a.task.Job.CreatedAt().Sub(b.task.Job.CreatedAt())
	if timeDiff != 0 {
		return timeDiff > 0
	}
	return a.Key() > b.Key()
}
