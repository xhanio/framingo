package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/utils/job"
)

// a task that outlives every test unless something ends it
const longCooldown = time.Hour

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// failingJob fails its first failures runs and then succeeds, counting every run.
func failingJob(id string, failures int32, runs *atomic.Int32) job.Job {
	return job.New(id, func(job.Context) error {
		if runs.Add(1) <= failures {
			return errors.Newf("run failed")
		}
		return nil
	})
}

func alwaysFailingJob(id string, runs *atomic.Int32) job.Job {
	return failingJob(id, 1<<30, runs)
}

func TestRequeueRerunsAFailedTaskUntilItSucceeds(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             failingJob("flaky", 2, &runs),
		RequeueAttempts: 5,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	eventually(t, "the finished task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	if n := runs.Load(); n != 3 {
		t.Fatalf("runs = %d, want 3: two failures and the success that ends the requeues", n)
	}
}

func TestRequeueGivesUpAfterItsAttempts(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("hopeless", &runs),
		RequeueAttempts: 2,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeues to run out", func() bool { return s.Stats(tk.ID()) == nil })
	if n := runs.Load(); n != 3 {
		t.Fatalf("runs = %d, want 3: the first run and two requeues", n)
	}
	if st := tk.Job.State(); st != job.StateFailed {
		t.Fatalf("state after the last attempt = %s, want %s", st, job.StateFailed)
	}
	if tk.IsRequeueing() {
		t.Fatal("a task out of attempts is still requeueing")
	}
}

func TestRequeueDoesNothingWithoutAttempts(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("no-requeue", &runs),
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
	if st := tk.Job.State(); st != job.StateFailed {
		t.Fatalf("state = %s, want %s", st, job.StateFailed)
	}
}

func TestRequeueComesOnTopOfTheJobRetry(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("both", &runs),
		RetryAttempts:   1,
		RetryDelay:      time.Millisecond,
		RequeueAttempts: 2,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeues to run out", func() bool { return s.Stats(tk.ID()) == nil })
	if n := runs.Load(); n != 6 {
		t.Fatalf("runs = %d, want 6: three executions of two runs each", n)
	}
}

func TestRequeueWaitsOutTheCooldown(t *testing.T) {
	const cooldown = 300 * time.Millisecond
	var (
		mu    sync.Mutex
		times []time.Time
	)
	tk := &Task{
		Job: job.New("patient", func(job.Context) error {
			mu.Lock()
			defer mu.Unlock()
			times = append(times, time.Now())
			if len(times) == 1 {
				return errors.Newf("run failed")
			}
			return nil
		}),
		RequeueAttempts: 1,
		RequeueCooldown: cooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	mu.Lock()
	defer mu.Unlock()
	if gap := times[1].Sub(times[0]); gap < cooldown {
		t.Fatalf("the rerun came %s after the failure, inside the %s cooldown", gap, cooldown)
	}
}

func TestRequeueCooldownFreesTheWorker(t *testing.T) {
	var failed atomic.Int32
	cooling := &Task{
		Job:             alwaysFailingJob("cooling", &failed),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	var ran atomic.Bool
	other := &Task{Job: job.New("other", func(job.Context) error {
		ran.Store(true)
		return nil
	})}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(cooling); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", cooling.IsRequeueing)
	if err := s.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to take another task", ran.Load)
}

func TestRequeueKeepsTheTaskRegisteredWhileCooling(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("registered", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	if st := tk.Job.State(); st != job.StateFailed {
		t.Fatalf("job state while the task waits = %s, want %s: the run did fail", st, job.StateFailed)
	}
	st := s.Stats(tk.ID())
	if st == nil {
		t.Fatal("a one-shot task waiting to run again lost its stats")
	}
	if st.Requeues != 1 {
		t.Fatalf("Requeues = %d, want 1", st.Requeues)
	}
	if st.Cooldown < longCooldown-time.Minute {
		t.Fatalf("Cooldown = %s, want about %s left", st.Cooldown, longCooldown)
	}
	if err := s.Add(tk); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of a task waiting to run again = %v, want Conflict", err)
	}
}

func TestRemoveEndsARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("removed", &runs),
		RequeueAttempts: 3,
		RequeueCooldown: 50 * time.Millisecond,
	}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	s.Remove(tk)
	if st := s.Stats(tk.ID()); st != nil {
		t.Fatalf("a removed task still has stats: %+v", st)
	}
	eventually(t, "the task to stop waiting", func() bool { return !tk.IsRequeueing() })
	time.Sleep(200 * time.Millisecond) // longer than the cooldown
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a removed task must not run again", n)
	}

	var again atomic.Int32
	next := &Task{Job: failingJob(tk.ID(), 0, &again)}
	if err := s.Add(next); err != nil {
		t.Fatalf("Add under the removed task's ID = %v", err)
	}
	eventually(t, "the new task to run", func() bool { return again.Load() == 1 })
}

func TestRemoveOfAQueuedRerunDoesNotRunIt(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	busy := &Task{Job: job.New("busy", func(job.Context) error {
		<-block
		return nil
	})}
	// the fetching goroutine holds this one at the handoff, leaving the rerun in the queue
	held := &Task{Job: job.New("held", func(job.Context) error { return nil }), Priority: -1}
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("queued", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: 200 * time.Millisecond,
	}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	if err := s.Add(busy); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busy.Job.IsState(job.StateRunning) })
	if err := s.Add(held); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fetching goroutine to hold a task", func() bool { return s.pq.IsEmpty() })
	eventually(t, "the rerun to be queued", func() bool { return !s.pq.IsEmpty() })
	s.Remove(tk)
	if !s.pq.IsEmpty() {
		t.Fatal("a removed task stayed in the queue")
	}
	eventually(t, "the task to stop waiting", func() bool { return !tk.IsRequeueing() })
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a removed task must not run again", n)
	}
}

func TestStopEndsARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("stopped", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	stopped := make(chan struct{})
	go func() {
		_ = s.Stop(true)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop(true) hung on a task waiting out its requeue cooldown")
	}
	if tk.IsRequeueing() {
		t.Fatal("a stopped task is still requeueing")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestTaskContextEndsARequeue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("ctx-canceled", &runs),
		Ctx:             ctx,
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	cancel()
	eventually(t, "the task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	if tk.IsRequeueing() {
		t.Fatal("a task whose context ended is still requeueing")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCanceledRunIsNotRequeued(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	tk := &Task{
		Job: job.New("canceled", func(jc job.Context) error {
			runs.Add(1)
			started <- struct{}{}
			<-jc.Context().Done()
			return jc.Context().Err()
		}),
		RequeueAttempts: 3,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	tk.Job.Cancel()
	eventually(t, "the canceled task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a canceled run is not a failure to requeue", n)
	}
	if st := tk.Job.State(); st != job.StateCanceled {
		t.Fatalf("state = %s, want %s", st, job.StateCanceled)
	}
}

func TestRunEndedByItsContextIsNotRequeued(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	tk := &Task{
		Job: job.New("ctx-run", func(jc job.Context) error {
			runs.Add(1)
			started <- struct{}{}
			<-jc.Context().Done()
			return jc.Context().Err()
		}),
		Ctx:             ctx,
		RequeueAttempts: 3,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	eventually(t, "the task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a run ended by its own context is not a failure to requeue", n)
	}
}

func TestTicksAreSkippedDuringARequeue(t *testing.T) {
	const cooldown = 2500 * time.Millisecond
	var (
		mu    sync.Mutex
		times []time.Time
	)
	tk := &Task{
		Job: job.New("tick-skipped", func(job.Context) error {
			mu.Lock()
			defer mu.Unlock()
			times = append(times, time.Now())
			if len(times) == 1 {
				return errors.Newf("run failed")
			}
			return nil
		}),
		Schedule:        "* * * * * *",
		RequeueAttempts: 1,
		RequeueCooldown: cooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	mu.Lock()
	defer mu.Unlock()
	if gap := times[1].Sub(times[0]); gap < cooldown {
		t.Fatalf("a tick ran the task %s after its failure, inside the %s requeue cooldown", gap, cooldown)
	}
}

func TestStatsRequeuesStartOverWithANewFiring(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             failingJob("fresh-firing", 1, &runs),
		Schedule:        "* * * * * *",
		RequeueAttempts: 1,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeue to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	if st := s.Stats(tk.ID()); st == nil || st.Requeues != 1 {
		t.Fatalf("stats after the requeue = %+v, want Requeues 1", st)
	}
	eventually(t, "the next firing to reset the count", func() bool {
		st := s.Stats(tk.ID())
		return runs.Load() >= 3 && st != nil && st.Requeues == 0
	})
}

// failedJob returns a job whose one run has failed.
func failedJob(t *testing.T, id string) job.Job {
	t.Helper()
	j := job.New(id, func(job.Context) error { return errors.Newf("run failed") })
	j.Run(context.Background(), nil)
	j.Wait()
	if !j.IsState(job.StateFailed) {
		t.Fatalf("job %s state = %s, want %s", id, j.State(), job.StateFailed)
	}
	return j
}

func TestQueueOrdersARequeuedTaskLikeANewOne(t *testing.T) {
	cases := []struct {
		name       string
		requeueing bool
		priority   int // of the task that ran, against a new task of priority 2
		wantFirst  string
	}{
		{name: "requeueing with the more urgent priority", requeueing: true, priority: 1, wantFirst: "ran"},
		{name: "requeueing with the less urgent priority", requeueing: true, priority: 3, wantFirst: "new"},
		{name: "requeueing with the same priority is the older", requeueing: true, priority: 2, wantFirst: "ran"},
		{name: "failed yields to a new task despite its priority", requeueing: false, priority: 1, wantFirst: "new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran := &Task{Job: failedJob(t, "ran"), Priority: tc.priority}
			ran.requeueing.Store(tc.requeueing)
			pq := staque.NewPriority(staque.WithLessFunc(priorityFunc))
			pq.Push(ran)
			pq.Push(&Task{Job: job.New("new", func(job.Context) error { return nil }), Priority: 2})
			if got := pq.MustPop().ID(); got != tc.wantFirst {
				t.Fatalf("first popped = %s, want %s", got, tc.wantFirst)
			}
		})
	}
}

// The queue orders by job state, so a task must not sit in it while its state
// changes: a queue out of order cannot find the task again, and Remove panics.
func TestTickDuringARunStaysOutOfTheQueue(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, 10)
	tk := &Task{
		Job: job.New("long", func(job.Context) error {
			started <- struct{}{}
			<-release
			return nil
		}),
		Schedule: "* * * * * *",
	}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	// with the only worker taken, a first tick is held at the handoff and a second stays queued
	time.Sleep(2200 * time.Millisecond)
	if !s.pq.IsEmpty() {
		t.Fatal("a tick during the run left the task in the queue")
	}
}

func TestStartingARunLeavesNoCopyInTheQueue(t *testing.T) {
	block := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	busy := &Task{Job: job.New("busy", func(job.Context) error {
		<-block
		return nil
	})}
	tk := &Task{
		Job: job.New("copied", func(job.Context) error {
			<-release
			return nil
		}),
		Manual: true,
	}
	// more urgent than the copy, so it is what the fetching goroutine takes once the handoff is done
	waiting := &Task{Job: job.New("waiting", func(job.Context) error { return nil }), Priority: -1}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(busy); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busy.Job.IsState(job.StateRunning) })
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fetching goroutine to hold the task", s.pq.IsEmpty)
	s.pq.Push(tk) // what a Remove and Add of the same task during the handoff leaves behind
	if err := s.Add(waiting); err != nil {
		t.Fatal(err)
	}
	close(block)
	eventually(t, "the task to run", func() bool { return tk.Job.IsState(job.StateRunning) })
	// the fetching goroutine holds waiting at the handoff: nothing else belongs in the queue
	eventually(t, "the copy to leave the queue", s.pq.IsEmpty)
}

func TestStrayPopDuringARequeueDoesNotRunTheTask(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("stray", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	// what the fetching goroutine can still be holding from before the run ended
	s.pq.Push(tk)
	eventually(t, "the stray pop to be taken", s.pq.IsEmpty)
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a stray pop must not cut the cooldown short", n)
	}
}

func TestTaskContextTakesAQueuedRerunOutOfTheQueue(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	busy := &Task{Job: job.New("busy", func(job.Context) error {
		<-block
		return nil
	})}
	// the fetching goroutine holds this one at the handoff, leaving the rerun in the queue
	held := &Task{Job: job.New("held", func(job.Context) error { return nil }), Priority: -1}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("ctx-queued", &runs),
		Ctx:             ctx,
		RequeueAttempts: 1,
		RequeueCooldown: 200 * time.Millisecond,
	}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	if err := s.Add(busy); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busy.Job.IsState(job.StateRunning) })
	if err := s.Add(held); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fetching goroutine to hold a task", s.pq.IsEmpty)
	eventually(t, "the rerun to be queued", func() bool { return !s.pq.IsEmpty() })
	cancel()
	eventually(t, "the task to stop waiting", func() bool { return !tk.IsRequeueing() })
	if !s.pq.IsEmpty() {
		t.Fatal("a task whose context ended stayed in the queue")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCanceledRerunEndsTheRequeue(t *testing.T) {
	var runs atomic.Int32
	rerunning := make(chan struct{}, 10)
	tk := &Task{
		Job: job.New("canceled-rerun", func(jc job.Context) error {
			if runs.Add(1) == 1 {
				return errors.Newf("run failed")
			}
			rerunning <- struct{}{}
			<-jc.Context().Done()
			return jc.Context().Err()
		}),
		RequeueAttempts: 3,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-rerunning
	tk.Job.Cancel()
	eventually(t, "the task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 2 {
		t.Fatalf("runs = %d, want 2: a canceled rerun ends the requeues", n)
	}
	if st := tk.Job.State(); st != job.StateCanceled {
		t.Fatalf("state = %s, want %s", st, job.StateCanceled)
	}
}

func TestAddAfterRemoveDuringARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("again", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", tk.IsRequeueing)
	s.Remove(tk)
	if err := s.Add(tk); err != nil {
		t.Fatalf("Add after Remove = %v", err)
	}
	eventually(t, "the task to run again", func() bool { return runs.Load() == 2 })
	eventually(t, "the new registration to start cooling", tk.IsRequeueing)
	time.Sleep(50 * time.Millisecond) // the first requeue must not undo the second as it ends
	if !tk.IsRequeueing() {
		t.Fatal("the removed registration's requeue ended the new one's")
	}
	if st := s.Stats(tk.ID()); st == nil || st.Requeues != 1 {
		t.Fatalf("stats = %+v, want Requeues 1", st)
	}
}

func TestARerunIsNotRequeueing(t *testing.T) {
	var runs atomic.Int32
	rerunning := make(chan struct{}, 10)
	release := make(chan struct{})
	tk := &Task{
		Job: job.New("rerun-state", func(job.Context) error {
			if runs.Add(1) == 1 {
				return errors.Newf("run failed")
			}
			rerunning <- struct{}{}
			<-release
			return nil
		}),
		RequeueAttempts: 1,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-rerunning
	defer close(release)
	if tk.IsRequeueing() {
		t.Fatal("a task running again is not waiting to run again")
	}
}

func TestRefusedStartIsNotRequeued(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	tk := &Task{
		Job: job.New("elsewhere", func(job.Context) error {
			runs.Add(1)
			close(started)
			<-release
			return nil
		}),
		RequeueAttempts: 3,
		RequeueCooldown: 10 * time.Millisecond,
	}
	tk.Job.Run(context.Background(), nil) // running before the manager ever sees it
	<-started
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the refused task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
	close(release)
	tk.Job.Wait()
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a refused start did not fail", n)
	}
}
