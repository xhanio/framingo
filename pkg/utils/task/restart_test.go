package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/utils/job"
)

// held starts a manager running a job that ignores its cancel until released.
func held(t *testing.T, concurrency int) (*manager, chan struct{}) {
	t.Helper()
	s := newManager(MaxConcurrency(concurrency))
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	tk := &Task{Job: holdJob("held", started, release), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	return s, release
}

func stopInBackground(s *manager, ctx context.Context) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		_ = s.Stop(ctx)
		close(stopped)
	}()
	return stopped
}

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// forceStop stops with an ended context, which leaves the held execution behind.
func forceStop(t *testing.T, s *manager) {
	t.Helper()
	if err := s.Stop(canceled()); !errors.Is(err, context.Canceled) {
		t.Fatalf("forced stop = %v, want the context's error for what it left", err)
	}
}

func assertWaiting(t *testing.T, stopped <-chan struct{}) {
	t.Helper()
	select {
	case <-stopped:
		t.Fatal("Stop returned with an execution still going")
	case <-time.After(100 * time.Millisecond):
	}
}

func assertReturns(t *testing.T, stopped <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-stopped:
	case <-time.After(within):
		t.Fatalf("Stop did not return within %s", within)
	}
}

func TestStopWaitsForTheExecutionsToEnd(t *testing.T) {
	s, release := held(t, 1)
	stopped := stopInBackground(s, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestAForcedStopReturnsAtOnce(t *testing.T) {
	s, release := held(t, 1)
	defer close(release)
	assertReturns(t, stopInBackground(s, canceled()), time.Second)
}

func TestAForcedStopWithNothingLeftReturnsNil(t *testing.T) {
	s := newManager()
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(canceled()); err != nil {
		t.Fatalf("forced stop = %v with nothing running", err)
	}
}

func TestStopGivesUpOnceItsContextEnds(t *testing.T) {
	s, release := held(t, 1)
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if err := s.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took > 2*time.Second {
		t.Fatalf("Stop took %s past its deadline", took)
	}
}

// stubborn ignores its cancel and ends only on the kill signal.
func stubborn(t *testing.T, s *manager) (killed <-chan struct{}) {
	t.Helper()
	started, kill, ended := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(ended) })
	tk := &Task{Job: job.New("stubborn", func(jc job.Context) error {
		close(started)
		select {
		case <-jc.Signaled(job.SignalKill):
			close(kill)
		case <-ended:
		}
		return nil
	}), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	return kill
}

func TestStopKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	s := startedManager(t)
	killed := stubborn(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	stopped := stopInBackground(s, ctx)
	select {
	case <-killed:
		t.Fatal("the kill came before the deadline")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("the deadline passed without a kill")
	}
	assertReturns(t, stopped, 5*time.Second)
}

func TestCancelingAStopGoingOnForcesIt(t *testing.T) {
	s := startedManager(t)
	killed := stubborn(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop(ctx) }()
	select {
	case err := <-stopped:
		t.Fatalf("Stop = %v before its context ended", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("canceling the stop did not kill what still ran")
	}
	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop = %v, want the cancel's error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return once canceled")
	}
}

func TestStopWaitingAfterAForcedStopWaits(t *testing.T) {
	s, release := held(t, 1)
	forceStop(t, s)
	stopped := stopInBackground(s, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestStartRightAfterAForcedStop(t *testing.T) {
	s, release := held(t, 1)
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	forceStop(t, s)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start = %v right after a forced stop", err)
	}
	var runs atomic.Int32
	if err := s.Add(&Task{Job: countingJob("next", &runs)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the new start to run a task while the one left over holds its slot", func() bool {
		return runs.Load() == 1
	})
}

func TestStopWaitingWaitsForWhatAForcedStopLeft(t *testing.T) {
	s, release := held(t, 1)
	forceStop(t, s)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := stopInBackground(s, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestAForcedStopKillsTheExecutions(t *testing.T) {
	s := startedManager(t)
	killed := stubborn(t, s)
	forceStop(t, s)
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("a forced stop did not send the execution the kill signal")
	}
}

func TestAStopThatWaitsDoesNotKill(t *testing.T) {
	s := newManager()
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	var killed atomic.Bool
	tk := &Task{Job: job.New("graceful", func(jc job.Context) error {
		close(started)
		<-jc.Context().Done()
		select {
		case <-jc.Signaled(job.SignalKill):
			killed.Store(true)
		default:
		}
		return nil
	}), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if killed.Load() {
		t.Fatal("a stop that waits sent the kill signal")
	}
}

func TestATaskAddedAgainRunsOnceWhatAForcedStopLeftEnds(t *testing.T) {
	s := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	tk := &Task{Job: job.New("again", func(job.Context) error {
		if runs.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})}
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	forceStop(t, s)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d while its last execution goes on, want 1", n)
	}
	free()
	eventually(t, "the task added again to run", func() bool { return runs.Load() == 2 })
}

func TestAForcedStopIsNotHeldUpByAnExclusiveTaskWaitingToRun(t *testing.T) {
	s, release := held(t, 2)
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	var runs atomic.Int32
	if err := s.Add(&Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to wait for the slots", s.pq.IsEmpty)
	assertReturns(t, stopInBackground(s, canceled()), time.Second)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start = %v right after a forced stop", err)
	}
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0: the exclusive task ran beside another", n)
	}
}

func TestAnExclusiveTaskRunsAlone(t *testing.T) {
	s := startedManager(t)
	firstStarted, firstRelease := make(chan struct{}), make(chan struct{})
	if err := s.Add(&Task{Job: holdJob("first", firstStarted, firstRelease)}); err != nil {
		t.Fatal(err)
	}
	<-firstStarted
	exclusiveStarted, exclusiveRelease := make(chan struct{}), make(chan struct{})
	if err := s.Add(&Task{Job: holdJob("exclusive", exclusiveStarted, exclusiveRelease), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	if err := s.Add(&Task{Job: countingJob("after", &runs), Priority: 1}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exclusiveStarted:
		t.Fatal("the exclusive task started beside another")
	case <-time.After(100 * time.Millisecond):
	}
	close(firstRelease)
	select {
	case <-exclusiveStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("the exclusive task did not start once alone")
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0: a task started beside the exclusive one", n)
	}
	close(exclusiveRelease)
	eventually(t, "the task after the exclusive one to run", func() bool { return runs.Load() == 1 })
}

func TestAnExclusiveTaskGivesBackEverySlot(t *testing.T) {
	s := startedManager(t, MaxConcurrency(2))
	var runs atomic.Int32
	if err := s.Add(&Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to run", func() bool { return runs.Load() == 1 })
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	firstStarted, secondStarted := make(chan struct{}), make(chan struct{})
	if err := s.Add(&Task{Job: holdJob("first", firstStarted, release)}, &Task{Job: holdJob("second", secondStarted, release)}); err != nil {
		t.Fatal(err)
	}
	for _, started := range []chan struct{}{firstStarted, secondStarted} {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("two tasks did not run at once after an exclusive one")
		}
	}
}

func TestAnExclusiveTaskDoesNotWaitForWhatAForcedStopLeft(t *testing.T) {
	s, release := held(t, 2)
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	forceStop(t, s)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	if err := s.Add(&Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to run beside what the forced stop left", func() bool {
		return runs.Load() == 1
	})
}

func TestStopLeavesTheQueueEmpty(t *testing.T) {
	for range 50 {
		s, release := held(t, 1)
		var runs atomic.Int32
		queued := &Task{Job: countingJob("queued", &runs), Manual: true}
		if err := s.Add(queued); err != nil {
			t.Fatal(err)
		}
		if err := s.Trigger(queued.ID(), nil); err != nil {
			t.Fatal(err)
		}
		forceStop(t, s)
		if !s.pq.IsEmpty() {
			t.Fatal("a forced stop left something in the queue")
		}
		close(release)
		if err := s.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !s.pq.IsEmpty() {
			t.Fatal("the stop left something in the queue")
		}
	}
}

func TestRemoveOfAStaleTaskLeavesItsSuccessorAlone(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		s := startedManager(t, MaxConcurrency(1))
		var staleRuns, runs atomic.Int32
		stale := &Task{Job: countingJob("same", &staleRuns), Manual: true}
		if err := s.Add(stale); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		successor := &Task{Job: countingJob("same", &runs), Manual: true}
		started, release := make(chan struct{}), make(chan struct{})
		blocker := &Task{Job: holdJob("blocker", started, release), Manual: true}
		if err := s.Add(successor, blocker); err != nil {
			t.Fatal(err)
		}
		if err := s.Trigger(blocker.ID(), nil); err != nil {
			t.Fatal(err)
		}
		<-started
		if err := s.Trigger(successor.ID(), nil); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		close(release)
		eventually(t, "the successor to run", func() bool { return runs.Load() == 1 })
	})
	t.Run("scheduled", func(t *testing.T) {
		s := startedManager(t)
		var staleRuns, runs atomic.Int32
		stale := &Task{Job: countingJob("same", &staleRuns), Schedule: "0 0 1 1 *"}
		if err := s.Add(stale); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		successor := &Task{Job: countingJob("same", &runs), Schedule: "* * * * * *"}
		if err := s.Add(successor); err != nil {
			t.Fatal(err)
		}
		s.Remove(stale)
		eventually(t, "the successor's schedule to run it", func() bool { return runs.Load() >= 1 })
	})
}

func TestCancelOfATaskWaitingForWhatAForcedStopLeftEndsIt(t *testing.T) {
	s := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	tk := &Task{Job: job.New("waiting", func(job.Context) error {
		if runs.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})}
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	forceStop(t, s)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to wait for its last execution", s.pq.IsEmpty)
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(tk.ID()); st != nil {
		t.Fatalf("a canceled one-shot task stayed registered: %+v", st)
	}
	free()
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a canceled task ran", n)
	}
}
