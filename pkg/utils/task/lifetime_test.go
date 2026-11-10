package task

import (
	"context"
	"testing"

	"github.com/xhanio/framingo/pkg/utils/job"
)

func startedScheduler(t *testing.T, opts ...Option) *manager {
	t.Helper()
	s := newScheduler(append([]Option{MaxConcurrency(3)}, opts...)...)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	return s
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
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(again.ID()); st != nil {
		t.Fatalf("a stopped manager still has stats for a task: %+v", st)
	}
}
