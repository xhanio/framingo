package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/log"
	"xhanio/framingo/pkg/util/strutil"
	"xhanio/framingo/pkg/util/task"
	"xhanio/framingo/pkg/util/task/executor"
)

func newTestTask(id string, d time.Duration, fail bool) task.Task {
	return task.New(id, func(tc task.Context) error {
		for {
			select {
			case <-tc.Context().Done():
				if tc.Context().Err() == context.DeadlineExceeded {
					log.Default.Debugf("task %s timed out", tc.ID())
					return errors.DeadlineExceeded.Newf("task timed out")
				} else {
					log.Default.Debugf("task %s canceled", tc.ID())
					return errors.Cancaled.Newf("task canceled")
				}
			case <-time.After(d):
				if fail {
					log.Default.Debugf("task %s failed", tc.ID())
					return errors.Newf("task %s failed", tc.ID())
				}
				log.Default.Debugf("task %s completed", tc.ID())
				return nil
			}
		}
	})
}

func TestMaxConcurrency(t *testing.T) {
	plans := make([]*Plan, 20)
	for i := range 20 {
		plans[i] = &Plan{
			Task:     newTestTask(fmt.Sprintf("#%d", i), 2*time.Second, false),
			Priority: i,
			Opts:     []executor.Option{
				// executor.WithTimeout(1 * time.Second),
			},
		}
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(20, func(i, j int) {
		plans[i], plans[j] = plans[j], plans[i]
	})
	s := newScheduler(MaxConcurrency(3))
	log.Default.Debugln("sequence:", strutil.Join(" ", plans[:10]...))
	s.Add(plans[:10]...)
	s.Start(context.Background())
	time.Sleep(3 * time.Second)
	log.Default.Debugln("sequence:", strutil.Join(" ", plans[10:]...))
	s.Add(plans[10:]...)
	time.Sleep(3 * time.Second)
	s.Stop(true)
}

func TestExclusivePlan(t *testing.T) {
	plans := make([]*Plan, 20)
	for i := range 20 {
		plans[i] = &Plan{
			Task:     newTestTask(fmt.Sprintf("#%d", i), 2*time.Second, false),
			Priority: i,
			Opts:     []executor.Option{
				// executor.WithTimeout(1 * time.Second),
			},
		}
		if i%5 == 0 {
			plans[i].Exclusive = true
		}
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(20, func(i, j int) {
		plans[i], plans[j] = plans[j], plans[i]
	})
	s := newScheduler(MaxConcurrency(3))
	log.Default.Debugln("sequence:", strutil.Join(" ", plans[:10]...))
	s.Add(plans[:10]...)
	s.Start(context.Background())
	time.Sleep(10 * time.Second)
	log.Default.Debugln("sequence:", strutil.Join(" ", plans[10:]...))
	s.Add(plans[10:]...)
	time.Sleep(10 * time.Second)
	s.Stop(true)
}
