package task

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestNoMoreThanMaxConcurrencyRunAtOnce(t *testing.T) {
	const slots, tasks = 3, 6
	m := startedManager(t, MaxConcurrency(slots))
	var (
		mu                  sync.Mutex
		running, peak, runs int
	)
	counts := func() (int, int, int) {
		mu.Lock()
		defer mu.Unlock()
		return running, peak, runs
	}
	release := make(chan struct{})
	for i := range tasks {
		tk := &entity.Task{Job: job.New(fmt.Sprintf("held-%d", i), func(jc job.Context) error {
			mu.Lock()
			running++
			peak = max(peak, running)
			mu.Unlock()
			select {
			case <-release:
			case <-jc.Context().Done():
			}
			mu.Lock()
			running--
			runs++
			mu.Unlock()
			return nil
		})}
		if err := m.Add(tk); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "the slots to fill", func() bool { running, _, _ := counts(); return running == slots })
	time.Sleep(50 * time.Millisecond) // time for a task over the limit to start
	close(release)
	eventually(t, "every task to run", func() bool { _, _, runs := counts(); return runs == tasks })
	if _, peak, _ := counts(); peak != slots {
		t.Fatalf("%d tasks ran at once, want %d", peak, slots)
	}
}

func TestStopForgetsTheTasks(t *testing.T) {
	m := startedManager(t)
	tk := &entity.Task{Job: job.New("yearly", func(job.Context) error { return nil }), Schedule: neverFires}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(tk.ID()); st != nil {
		t.Fatalf("a stopped manager still has stats for a task: %+v", st)
	}
}

func TestAnExclusiveTaskRunsAlone(t *testing.T) {
	m := startedManager(t)
	firstStarted, firstRelease := make(chan struct{}), make(chan struct{})
	if err := m.Add(&entity.Task{Job: holdJob("first", firstStarted, firstRelease)}); err != nil {
		t.Fatal(err)
	}
	<-firstStarted
	exclusiveStarted, exclusiveRelease := make(chan struct{}), make(chan struct{})
	if err := m.Add(&entity.Task{Job: holdJob("exclusive", exclusiveStarted, exclusiveRelease), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	if err := m.Add(&entity.Task{Job: countingJob("after", &runs), Priority: 1}); err != nil {
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
	m := startedManager(t, MaxConcurrency(2))
	var runs atomic.Int32
	if err := m.Add(&entity.Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to run", func() bool { return runs.Load() == 1 })
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	firstStarted, secondStarted := make(chan struct{}), make(chan struct{})
	if err := m.Add(&entity.Task{Job: holdJob("first", firstStarted, release)}, &entity.Task{Job: holdJob("second", secondStarted, release)}); err != nil {
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

func TestAnExclusiveTaskDroppedWhileItWaitsHoldsUpNothing(t *testing.T) {
	cases := []struct {
		name string
		drop func(*manager, *entity.Task) error
	}{
		{name: "canceled", drop: func(m *manager, tk *entity.Task) error { return m.Cancel(tk.ID()) }},
		{name: "removed", drop: func(m *manager, tk *entity.Task) error { return m.Remove(context.Background(), tk) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := startedManager(t, MaxConcurrency(2))
			started, release := make(chan struct{}), make(chan struct{})
			t.Cleanup(func() { close(release) })
			if err := m.Add(&entity.Task{Job: holdJob("long", started, release)}); err != nil {
				t.Fatal(err)
			}
			<-started
			var exclusiveRuns, runs atomic.Int32
			exclusive := &entity.Task{Job: countingJob("exclusive", &exclusiveRuns), Manual: true, Exclusive: true}
			if err := m.Add(exclusive); err != nil {
				t.Fatal(err)
			}
			if err := m.Trigger(exclusive.ID(), nil); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the exclusive task to wait for the slots", m.pq.IsEmpty)
			if err := tc.drop(m, exclusive); err != nil {
				t.Fatal(err)
			}
			if err := m.Add(&entity.Task{Job: countingJob("next", &runs)}); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the next task to run beside the long one", func() bool { return runs.Load() == 1 })
			if n := exclusiveRuns.Load(); n != 0 {
				t.Fatalf("the dropped exclusive task ran %d times", n)
			}
		})
	}
}

func TestQuickTasksDoNotUnbalanceTheQueue(t *testing.T) {
	const n = 3000
	var done atomic.Int32
	m := newManager(MaxConcurrency(3))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(context.Background()) }()
	for i := range n {
		tk := &entity.Task{Job: job.New(fmt.Sprintf("quick-%d", i), func(job.Context) error {
			done.Add(1)
			return nil
		})}
		if err := m.Add(tk); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for done.Load() < n && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := done.Load(); got != n {
		t.Fatalf("%d of %d quick tasks finished", got, n)
	}
}

func TestStartingARunLeavesNoCopyInTheQueue(t *testing.T) {
	block := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	blocker := &entity.Task{Job: job.New("blocker", func(job.Context) error {
		<-block
		return nil
	})}
	// popped, and out of the queue until blocker gives up its slot
	tk := &entity.Task{
		Job: job.New("copied", func(job.Context) error {
			<-release
			return nil
		}),
		Manual:    true,
		Exclusive: true,
	}
	m := startedManager(t, MaxConcurrency(2))
	if err := m.Add(blocker); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return blocker.Job.IsState(job.StateRunning) })
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to wait for its slots", m.pq.IsEmpty)
	m.pq.Push(registrationOf(m, tk)) // what a Cancel and a Trigger meanwhile leave behind
	close(block)
	eventually(t, "the task to run", func() bool { return tk.Job.IsState(job.StateRunning) })
	if !m.pq.IsEmpty() {
		t.Fatal("the copy stayed in the queue")
	}
}

// held starts a manager running a job that ignores its cancel until released.
func held(t *testing.T, concurrency int) (*manager, chan struct{}) {
	t.Helper()
	m := newManager(MaxConcurrency(concurrency))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	tk := &entity.Task{Job: holdJob("held", started, release), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	return m, release
}

func stopInBackground(m *manager, ctx context.Context) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		_ = m.Stop(ctx)
		close(stopped)
	}()
	return stopped
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
	m, release := held(t, 1)
	stopped := stopInBackground(m, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestAStopThatWaitsDoesNotKill(t *testing.T) {
	m := newManager()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	var killed atomic.Bool
	tk := &entity.Task{Job: job.New("graceful", func(jc job.Context) error {
		close(started)
		<-jc.Context().Done()
		select {
		case <-jc.Signaled(job.SignalKill):
			killed.Store(true)
		default:
		}
		return nil
	}), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if killed.Load() {
		t.Fatal("a stop that waits sent the kill signal")
	}
}

func TestStopInterruptsARetryWait(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:           alwaysFailingJob("stop-retry", &runs),
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
	m := newManager(MaxConcurrency(1))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond) // past the retry the stop must have ended
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a stopped manager must not retry", n)
	}
}

func TestStopLeavesTheQueueEmpty(t *testing.T) {
	for range 50 {
		m, release := held(t, 1)
		var runs atomic.Int32
		queued := &entity.Task{Job: countingJob("queued", &runs), Manual: true}
		if err := m.Add(queued); err != nil {
			t.Fatal(err)
		}
		if err := m.Trigger(queued.ID(), nil); err != nil {
			t.Fatal(err)
		}
		forceStop(t, m)
		if !m.pq.IsEmpty() {
			t.Fatal("a forced stop left something in the queue")
		}
		close(release)
		if err := m.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !m.pq.IsEmpty() {
			t.Fatal("the stop left something in the queue")
		}
	}
}

func TestStopGivesUpOnceItsContextEnds(t *testing.T) {
	m, release := held(t, 1)
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if err := m.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took > 2*time.Second {
		t.Fatalf("Stop took %s past its deadline", took)
	}
}

func TestStopKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	m := startedManager(t)
	_, killed := stubborn(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	stopped := stopInBackground(m, ctx)
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
	m := startedManager(t)
	_, killed := stubborn(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- m.Stop(ctx) }()
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

func TestAForcedStopReturnsAtOnce(t *testing.T) {
	m, release := held(t, 1)
	defer close(release)
	assertReturns(t, stopInBackground(m, canceled()), time.Second)
}

func TestAForcedStopWithNothingLeftReturnsNil(t *testing.T) {
	m := newManager()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(canceled()); err != nil {
		t.Fatalf("forced stop = %v with nothing running", err)
	}
}

func TestAForcedStopKillsTheExecutions(t *testing.T) {
	m := startedManager(t)
	_, killed := stubborn(t, m)
	forceStop(t, m)
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("a forced stop did not send the execution the kill signal")
	}
}

func TestAForcedStopIsNotHeldUpByAnExclusiveTaskWaitingToRun(t *testing.T) {
	m, release := held(t, 2)
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	var runs atomic.Int32
	if err := m.Add(&entity.Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to wait for the slots", m.pq.IsEmpty)
	assertReturns(t, stopInBackground(m, canceled()), time.Second)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start = %v right after a forced stop", err)
	}
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0: the exclusive task ran beside another", n)
	}
}

func TestStopWaitingAfterAForcedStopWaits(t *testing.T) {
	m, release := held(t, 1)
	forceStop(t, m)
	stopped := stopInBackground(m, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestStartRightAfterAForcedStop(t *testing.T) {
	m, release := held(t, 1)
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	forceStop(t, m)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start = %v right after a forced stop", err)
	}
	var runs atomic.Int32
	if err := m.Add(&entity.Task{Job: countingJob("next", &runs)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the new start to run a task while the one left over holds its slot", func() bool {
		return runs.Load() == 1
	})
}

func TestStopWaitingWaitsForWhatAForcedStopLeft(t *testing.T) {
	m, release := held(t, 1)
	forceStop(t, m)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := stopInBackground(m, context.Background())
	assertWaiting(t, stopped)
	close(release)
	assertReturns(t, stopped, 5*time.Second)
}

func TestATaskAddedAgainRunsOnceWhatAForcedStopLeftEnds(t *testing.T) {
	m := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	tk := &entity.Task{Job: job.New("again", func(job.Context) error {
		if runs.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})}
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	forceStop(t, m)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d while its last execution goes on, want 1", n)
	}
	free()
	eventually(t, "the task added again to run", func() bool { return runs.Load() == 2 })
}

func TestAnExclusiveTaskDoesNotWaitForWhatAForcedStopLeft(t *testing.T) {
	m, release := held(t, 2)
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	t.Cleanup(func() { close(release) })
	forceStop(t, m)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	if err := m.Add(&entity.Task{Job: countingJob("exclusive", &runs), Exclusive: true}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the exclusive task to run beside what the forced stop left", func() bool {
		return runs.Load() == 1
	})
}
