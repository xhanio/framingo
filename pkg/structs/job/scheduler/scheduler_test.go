package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/structs/job"
	"github.com/xhanio/framingo/pkg/structs/job/executor"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/strutil"
)

func newTestJob(id string, d time.Duration, fail bool) job.Job {
	return job.New(id, func(tc job.Context) error {
		for {
			select {
			case <-tc.Context().Done():
				if tc.Context().Err() == context.DeadlineExceeded {
					log.Default.Debugf("job %s timed out", tc.ID())
					return errors.DeadlineExceeded.Newf("job timed out")
				} else {
					log.Default.Debugf("job %s canceled", tc.ID())
					return errors.Cancaled.Newf("job canceled")
				}
			case <-time.After(d):
				if fail {
					log.Default.Debugf("job %s failed", tc.ID())
					return errors.Newf("job %s failed", tc.ID())
				}
				log.Default.Debugf("job %s completed", tc.ID())
				return nil
			}
		}
	})
}

func TestMaxConcurrency(t *testing.T) {
	plans := make([]*Plan, 20)
	for i := range 20 {
		plans[i] = &Plan{
			Job:      newTestJob(fmt.Sprintf("#%d", i), 2*time.Second, false),
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
			Job:      newTestJob(fmt.Sprintf("#%d", i), 2*time.Second, false),
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
