package task

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/strutil"
)

func newTestJob(id string, d time.Duration, fail bool) job.Job {
	return job.New(id, func(tc job.Context) error {
		for {
			select {
			case <-tc.Context().Done():
				if tc.Context().Err() == context.DeadlineExceeded {
					return errors.DeadlineExceeded.Newf("job timed out")
				} else {
					return errors.Cancaled.Newf("job canceled")
				}
			case <-time.After(d):
				if fail {
					return errors.Newf("job %s failed", tc.ID())
				}
				return nil
			}
		}
	})
}

func TestMaxConcurrency(t *testing.T) {
	tasks := make([]*Task, 20)
	for i := range 20 {
		tasks[i] = &Task{
			Job:      newTestJob(fmt.Sprintf("#%d", i), 2*time.Second, false),
			Priority: i,
		}
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(20, func(i, j int) {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	})
	s := newScheduler(MaxConcurrency(3))
	log.Default.Debugln("sequence:", strutil.Join(" ", tasks[:10]...))
	_ = s.Add(tasks[:10]...)
	_ = s.Start(context.Background())
	time.Sleep(3 * time.Second)
	log.Default.Debugln("sequence:", strutil.Join(" ", tasks[10:]...))
	_ = s.Add(tasks[10:]...)
	time.Sleep(3 * time.Second)
	_ = s.Stop(true)
}

func TestExclusivePlan(t *testing.T) {
	tasks := make([]*Task, 20)
	for i := range 20 {
		tasks[i] = &Task{
			Job:      newTestJob(fmt.Sprintf("#%d", i), 1*time.Second, false),
			Priority: i,
		}
		if i%5 == 0 {
			tasks[i].Exclusive = true
		}
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(20, func(i, j int) {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	})
	s := newScheduler(MaxConcurrency(3))
	log.Default.Debugln("sequence:", strutil.Join(" ", tasks[:10]...))
	_ = s.Add(tasks[:10]...)
	_ = s.Start(context.Background())
	time.Sleep(5 * time.Second)
	log.Default.Debugln("sequence:", strutil.Join(" ", tasks[10:]...))
	_ = s.Add(tasks[10:]...)
	time.Sleep(5 * time.Second)
	_ = s.Stop(true)
	time.Sleep(1 * time.Second)
}

func TestPriorityQueue(t *testing.T) {
	pq := staque.NewPriority(
		staque.WithLessFunc(priorityFunc),
		staque.BlockIfEmpty[*Task](),
	)
	tasks := make([]*Task, 20)
	for i := range 20 {
		tasks[i] = &Task{
			Job:      newTestJob(fmt.Sprintf("#%d", i), 2*time.Second, false),
			Priority: i,
		}
		if i%5 == 0 {
			tasks[i].Exclusive = true
		}
	}
	pq.Push(exiting)
	pq.Push(tasks...)
	pq.Push(exiting)
	table := printutil.NewTable(os.Stdout)
	for range 21 {
		item := pq.MustPop()
		table.Row(item.Key())
	}
	table.Flush()
}
