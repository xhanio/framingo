package task

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestTickOverlappingARunIsSkipped(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	tk := &entity.Task{
		Job: job.New("overlap", func(job.Context) error {
			runs.Add(1)
			started <- struct{}{}
			<-release
			return nil
		}),
		Schedule:      "* * * * * *",
		RetryAttempts: 5,
		RetryDelay:    20 * time.Millisecond,
	}
	s := startedManager(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("the task never fired")
	}
	time.Sleep(2200 * time.Millisecond) // two more ticks while the first run is still going
	if n := runs.Load(); n != 1 {
		t.Fatalf("task ran %d times while its first run was going, want 1", n)
	}
	if s.Stats(tk.ID()) == nil {
		t.Fatal("the running execution lost its stats to an overlapping tick")
	}
	close(release)
}

func TestAddRejectsTaskIDAlreadyAdded(t *testing.T) {
	s := startedManager(t)
	first := &entity.Task{Job: job.New("dup", func(job.Context) error { return nil }), Schedule: "0 0 1 1 *"}
	second := &entity.Task{Job: job.New("dup", func(job.Context) error { return nil }), Schedule: "0 0 1 1 *"}
	if err := s.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(second); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of an ID already added = %v, want Conflict", err)
	}
	if err := s.Add(first); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of the same task twice = %v, want Conflict", err)
	}
}
