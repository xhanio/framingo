package task

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job/executor"
)

var (
	_ common.Unique   = (*registration)(nil)
	_ common.Weighted = (*registration)(nil)
)

// registration lasts from Add to Remove; its executor outlives a single execution so that retry
// state carries over from one push to the next.
type registration struct {
	task *Task
	exec executor.Executor

	// guarded by the manager's el
	cron     cron.EntryID // its schedule's entry, 0 without one
	queued   bool         // waiting to run: in the queue, popped, or held back by its task's last execution
	params   any          // what Trigger gave this firing, in place of Task.Params
	requeues uint         // times the current firing went back to the queue
	cooldown *time.Timer  // set while a failed execution cools down
	cooling  time.Time    // when the cooldown ends
	unwatch  func() bool  // set while a rerun waits, which Task.Ctx ending stops
	priority int          // the task's Priority, as the queue orders it
	seq      uint64       // its latest arrival in the queue
}

// requeueing: a failed execution waits to run again, cooling down or back in the queue.
func (r *registration) requeueing() bool {
	return r.cooldown != nil || (r.queued && r.requeues > 0)
}

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
	case !a.valid(): // a wake-up, which goes first
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
