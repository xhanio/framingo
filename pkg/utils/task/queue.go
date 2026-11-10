package task

import "github.com/xhanio/framingo/pkg/types/common"

var (
	_ common.Unique   = (*registration)(nil)
	_ common.Weighted = (*registration)(nil)
)

// exiting unblocks the fetching goroutine as the manager stops; it leaves the queue first.
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

// before reads only what is fixed while a task is queued, which keeps the queue in order.
func before(a, b *registration) bool {
	switch {
	case !a.valid():
		return b.valid()
	case !b.valid():
		return false
	case a.priority != b.priority:
		return a.priority < b.priority
	case a.seq != b.seq:
		return a.seq < b.seq
	default:
		return a.Key() < b.Key()
	}
}

// queueOrder is the queue's less function: Pop takes the greatest, so the first out sorts last.
func queueOrder(a, b *registration) bool {
	return before(b, a)
}
