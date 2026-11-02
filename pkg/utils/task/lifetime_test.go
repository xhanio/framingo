package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
)

func startedScheduler(t *testing.T, opts ...Option) *manager {
	t.Helper()
	s := newScheduler(append([]Option{MaxConcurrency(3)}, opts...)...)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop(true) })
	return s
}

func TestCooldownHoldsAcrossCronTicks(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	tk := &Task{
		Job: job.New("cooled", func(job.Context) error {
			runs.Add(1)
			started <- struct{}{}
			return nil
		}),
		Schedule: "* * * * * *",
		Cooldown: time.Hour,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("the task never fired")
	}
	time.Sleep(2500 * time.Millisecond) // two more ticks, both inside the cooldown
	if n := runs.Load(); n != 1 {
		t.Fatalf("task ran %d times, want 1: ticks inside the cooldown must not run it", n)
	}
	st := s.Stats(tk.ID())
	if st == nil {
		t.Fatal("a scheduled task between runs must have stats")
	}
	if st.Cooldown < 59*time.Minute {
		t.Fatalf("Cooldown = %s, want about an hour left", st.Cooldown)
	}
}

func TestRemoveAndStopForgetTheTask(t *testing.T) {
	s := startedScheduler(t)
	tk := &Task{Job: job.New("yearly", func(job.Context) error { return nil }), Schedule: "0 0 1 1 *"}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if s.Stats(tk.ID()) == nil {
		t.Fatal("an added scheduled task must have stats before it ever runs")
	}
	s.Remove(tk)
	if st := s.Stats(tk.ID()); st != nil {
		t.Fatalf("a removed task still has stats: %+v", st)
	}

	again := &Task{Job: job.New("yearly-2", func(job.Context) error { return nil }), Schedule: "0 0 1 1 *"}
	if err := s.Add(again); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(true); err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(again.ID()); st != nil {
		t.Fatalf("a stopped manager still has stats for a task: %+v", st)
	}
}
