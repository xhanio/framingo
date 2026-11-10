package task

import (
	"context"
	"errors"
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

func removeInBackground(s *manager, ctx context.Context, tasks ...*Task) <-chan error {
	removed := make(chan error, 1)
	go func() { removed <- s.Remove(ctx, tasks...) }()
	return removed
}

func assertStillRemoving(t *testing.T, removed <-chan error) {
	t.Helper()
	select {
	case err := <-removed:
		t.Fatalf("Remove = %v while the execution is still going", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func assertRemoved(t *testing.T, removed <-chan error) {
	t.Helper()
	select {
	case err := <-removed:
		if err != nil {
			t.Fatalf("Remove = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Remove never returned")
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
			s := newManager(tc.opts...)
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
	s := startedManager(t)
	var runs atomic.Int32
	tk := &Task{Job: countingJob("idle", &runs), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	assertRemoved(t, removeInBackground(s, context.Background(), tk))
}

func TestRemoveIsDoneOnceTheExecutionHasEnded(t *testing.T) {
	s := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	tk := &Task{Job: holdJob("held", started, release), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	removed := removeInBackground(s, context.Background(), tk)
	assertStillRemoving(t, removed)
	close(release)
	assertRemoved(t, removed)
	if !tk.Job.IsDone() {
		t.Fatalf("job state = %s once Remove returned", tk.Job.State())
	}
}

func TestRemoveOfSeveralIsDoneOnceTheLastHasEnded(t *testing.T) {
	s := startedManager(t)
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
	removed := removeInBackground(s, context.Background(), first, second)
	close(firstRelease)
	assertStillRemoving(t, removed)
	close(secondRelease)
	assertRemoved(t, removed)
}

func TestQueuedTaskRemovedIsDoneAtOnceAndNeverRuns(t *testing.T) {
	s := startedManager(t, MaxConcurrency(1))
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
	assertRemoved(t, removeInBackground(s, context.Background(), queued))
	close(release)
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("the removed task ran %d times", n)
	}
}

func TestTaskRemovedDuringARequeueNeverRunsAgain(t *testing.T) {
	s := startedManager(t)
	var runs atomic.Int32
	tk := &Task{Job: alwaysFailingJob("requeued", &runs), Manual: true, RequeueAttempts: 100, RequeueCooldown: 10 * time.Millisecond}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a requeue", func() bool { return runs.Load() >= 2 })
	assertRemoved(t, removeInBackground(s, context.Background(), tk))
	n := runs.Load()
	time.Sleep(100 * time.Millisecond)
	if got := runs.Load(); got != n {
		t.Fatalf("runs went from %d to %d after the end was reported", n, got)
	}
}

func TestRemoveKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	s := startedManager(t)
	tk, killed := stubborn(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if err := s.Remove(ctx, tk); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Remove = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took < 100*time.Millisecond {
		t.Fatalf("Remove gave up after %s, before its deadline", took)
	}
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("the deadline passed without a kill")
	}
}

func TestARemoveWithAnEndedContextKillsAtOnce(t *testing.T) {
	s := startedManager(t)
	tk, killed := stubborn(t, s)
	if err := s.Remove(canceled(), tk); !errors.Is(err, context.Canceled) {
		t.Fatalf("Remove = %v, want the context's error for what it left", err)
	}
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("a forced remove did not kill")
	}
}

func TestRemoveKillsOnlyWhatItRemoves(t *testing.T) {
	s := startedManager(t)
	tk, _ := stubborn(t, s)
	otherStarted, otherRelease := make(chan struct{}), make(chan struct{})
	defer close(otherRelease)
	var otherKilled atomic.Bool
	other := &Task{Job: job.New("other", func(jc job.Context) error {
		close(otherStarted)
		select {
		case <-jc.Signaled(job.SignalKill):
			otherKilled.Store(true)
		case <-otherRelease:
		}
		return nil
	}), Manual: true}
	if err := s.Add(other); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(other.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-otherStarted
	_ = s.Remove(canceled(), tk)
	time.Sleep(50 * time.Millisecond)
	if otherKilled.Load() {
		t.Fatal("Remove killed a task it did not remove")
	}
}
