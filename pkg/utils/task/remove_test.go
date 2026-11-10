package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
)

// holdJob ignores its context: it ends when released, however it was asked to stop.
func holdJob(id string, started chan<- struct{}, release <-chan struct{}) job.Job {
	return job.New(id, func(job.Context) error {
		close(started)
		<-release
		return nil
	})
}

func assertOpen(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("Remove reported the end while the execution is still going")
	case <-time.After(50 * time.Millisecond):
	}
}

func assertClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Remove never reported the end of the execution")
	}
}

func TestNewWithoutAConcurrencyStillRunsTasks(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{name: "not set"},
		{name: "zero", opts: []Option{MaxConcurrency(0)}},
		{name: "negative", opts: []Option{MaxConcurrency(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newScheduler(tc.opts...)
			if err := s.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Stop(context.Background()) })
			if s.concurrent != defaultConcurrency {
				t.Fatalf("concurrency = %d, want %d", s.concurrent, defaultConcurrency)
			}
			var runs atomic.Int32
			if err := s.Add(&Task{Job: countingJob("once", &runs)}); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the task to run", func() bool { return runs.Load() == 1 })
		})
	}
}

func TestGraceDefaultsToTenSeconds(t *testing.T) {
	cases := []struct {
		period time.Duration
		want   time.Duration
	}{
		{period: 0, want: 10 * time.Second},
		{period: -time.Second, want: 10 * time.Second},
		{period: time.Second, want: time.Second},
	}
	for _, tc := range cases {
		if got := (&Task{GracePeriod: tc.period}).Grace(); got != tc.want {
			t.Errorf("Grace of %s = %s, want %s", tc.period, got, tc.want)
		}
	}
}

func TestRemoveOfATaskNotRunningIsDoneAtOnce(t *testing.T) {
	s := startedScheduler(t)
	var runs atomic.Int32
	tk := &Task{Job: countingJob("idle", &runs), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	assertClosed(t, s.Remove(tk))
}

func TestRemoveIsDoneOnceTheExecutionHasEnded(t *testing.T) {
	s := startedScheduler(t)
	started, release := make(chan struct{}), make(chan struct{})
	tk := &Task{Job: holdJob("held", started, release), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	done := s.Remove(tk)
	assertOpen(t, done)
	close(release)
	assertClosed(t, done)
	if !tk.Job.IsDone() {
		t.Fatalf("job state = %s once Remove reported the end", tk.Job.State())
	}
}

func TestRemoveOfSeveralIsDoneOnceTheLastHasEnded(t *testing.T) {
	s := startedScheduler(t)
	firstStarted, firstRelease := make(chan struct{}), make(chan struct{})
	secondStarted, secondRelease := make(chan struct{}), make(chan struct{})
	first := &Task{Job: holdJob("first", firstStarted, firstRelease), Manual: true}
	second := &Task{Job: holdJob("second", secondStarted, secondRelease), Manual: true}
	if err := s.Add(first, second); err != nil {
		t.Fatal(err)
	}
	for _, tk := range []*Task{first, second} {
		if err := s.Trigger(tk.ID(), nil); err != nil {
			t.Fatal(err)
		}
	}
	<-firstStarted
	<-secondStarted
	done := s.Remove(first, second)
	close(firstRelease)
	assertOpen(t, done)
	close(secondRelease)
	assertClosed(t, done)
}

func TestQueuedTaskRemovedIsDoneAtOnceAndNeverRuns(t *testing.T) {
	s := startedScheduler(t, MaxConcurrency(1))
	started, release := make(chan struct{}), make(chan struct{})
	blocker := &Task{Job: holdJob("blocker", started, release), Manual: true}
	var runs atomic.Int32
	queued := &Task{Job: countingJob("queued", &runs), Manual: true}
	if err := s.Add(blocker, queued); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(blocker.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := s.Trigger(queued.ID(), nil); err != nil {
		t.Fatal(err)
	}
	assertClosed(t, s.Remove(queued))
	close(release)
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("the removed task ran %d times", n)
	}
}

func TestTaskRemovedDuringARequeueNeverRunsAgain(t *testing.T) {
	s := startedScheduler(t)
	var runs atomic.Int32
	tk := &Task{Job: alwaysFailingJob("requeued", &runs), Manual: true, RequeueAttempts: 100, RequeueCooldown: 10 * time.Millisecond}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a requeue", func() bool { return runs.Load() >= 2 })
	assertClosed(t, s.Remove(tk))
	n := runs.Load()
	time.Sleep(100 * time.Millisecond)
	if got := runs.Load(); got != n {
		t.Fatalf("runs went from %d to %d after the end was reported", n, got)
	}
}
