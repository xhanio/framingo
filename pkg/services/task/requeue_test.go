package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestRequeueRerunsAFailedTaskUntilItSucceeds(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             failingJob("flaky", 2, &runs),
		RequeueAttempts: 5,
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	eventually(t, "the finished task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	if n := runs.Load(); n != 3 {
		t.Fatalf("runs = %d, want 3: two failures and the success that ends the requeues", n)
	}
}

func TestRequeueGivesUpAfterItsAttempts(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("hopeless", &runs),
		RequeueAttempts: 2,
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeues to run out", func() bool { return m.Stats(tk.ID()) == nil })
	if n := runs.Load(); n != 3 {
		t.Fatalf("runs = %d, want 3: the first run and two requeues", n)
	}
	if st := tk.Job.State(); st != job.StateFailed {
		t.Fatalf("state after the last attempt = %s, want %s", st, job.StateFailed)
	}
	if requeueing(m, tk)() {
		t.Fatal("a task out of attempts is still requeueing")
	}
}

func TestRequeueDoesNothingWithoutAttempts(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("no-requeue", &runs),
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
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
	tk := &entity.Task{
		Job:             alwaysFailingJob("both", &runs),
		RetryAttempts:   1,
		RetryDelay:      time.Millisecond,
		RequeueAttempts: 2,
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeues to run out", func() bool { return m.Stats(tk.ID()) == nil })
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
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	mu.Lock()
	defer mu.Unlock()
	if gap := times[1].Sub(times[0]); gap < cooldown {
		t.Fatalf("the rerun came %s after the failure, inside the %s cooldown", gap, cooldown)
	}
}

func TestRequeueCooldownFreesTheSlot(t *testing.T) {
	var failed atomic.Int32
	cooling := &entity.Task{
		Job:             alwaysFailingJob("cooling", &failed),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	var ran atomic.Bool
	other := &entity.Task{Job: job.New("other", func(job.Context) error {
		ran.Store(true)
		return nil
	})}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(cooling); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, cooling))
	if err := m.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to take another task", ran.Load)
}

func TestRequeueKeepsTheTaskRegisteredWhileCooling(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("registered", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if st := tk.Job.State(); st != job.StateFailed {
		t.Fatalf("job state while the task waits = %s, want %s: the run did fail", st, job.StateFailed)
	}
	st := m.Stats(tk.ID())
	if st == nil {
		t.Fatal("a one-shot task waiting to run again lost its stats")
	}
	if st.Requeues != 1 {
		t.Fatalf("Requeues = %d, want 1", st.Requeues)
	}
	if st.Cooldown < longCooldown-time.Minute {
		t.Fatalf("Cooldown = %s, want about %s left", st.Cooldown, longCooldown)
	}
	if err := m.Add(tk); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of a task waiting to run again = %v, want Conflict", err)
	}
}

func TestARerunIsNotRequeueing(t *testing.T) {
	var runs atomic.Int32
	rerunning := make(chan struct{}, 10)
	release := make(chan struct{})
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-rerunning
	defer close(release)
	if requeueing(m, tk)() {
		t.Fatal("a task running again is not waiting to run again")
	}
}

func TestStatsRequeuesStartOverWithANewFiring(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             failingJob("fresh-firing", 1, &runs),
		Schedule:        "* * * * * *",
		RequeueAttempts: 1,
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeue to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	if st := m.Stats(tk.ID()); st == nil || st.Requeues != 1 {
		t.Fatalf("stats after the requeue = %+v, want Requeues 1", st)
	}
	eventually(t, "the next firing to reset the count", func() bool {
		st := m.Stats(tk.ID())
		return runs.Load() >= 3 && st != nil && st.Requeues == 0
	})
}

func TestTicksAreSkippedDuringARequeue(t *testing.T) {
	const cooldown = 2500 * time.Millisecond
	var (
		mu    sync.Mutex
		times []time.Time
	)
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to succeed", func() bool { return tk.Job.IsState(job.StateSucceeded) })
	mu.Lock()
	defer mu.Unlock()
	if gap := times[1].Sub(times[0]); gap < cooldown {
		t.Fatalf("a tick ran the task %s after its failure, inside the %s requeue cooldown", gap, cooldown)
	}
}

func TestStrayPopDuringARequeueDoesNotRunTheTask(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("stray", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	// a stray entry of the task, which must not cut its cooldown short
	m.pq.Push(registrationOf(m, tk))
	eventually(t, "the stray pop to be taken", m.pq.IsEmpty)
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a stray pop must not cut the cooldown short", n)
	}
}

func TestCanceledRunIsNotRequeued(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	tk := &entity.Task{
		Job: job.New("canceled", func(jc job.Context) error {
			runs.Add(1)
			started <- struct{}{}
			<-jc.Context().Done()
			return jc.Context().Err()
		}),
		RequeueAttempts: 3,
		RequeueCooldown: 10 * time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	tk.Job.Cancel()
	eventually(t, "the canceled task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a canceled run is not a failure to requeue", n)
	}
	if st := tk.Job.State(); st != job.StateCanceled {
		t.Fatalf("state = %s, want %s", st, job.StateCanceled)
	}
}

func TestCanceledRerunEndsTheRequeue(t *testing.T) {
	var runs atomic.Int32
	rerunning := make(chan struct{}, 10)
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-rerunning
	tk.Job.Cancel()
	eventually(t, "the task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 2 {
		t.Fatalf("runs = %d, want 2: a canceled rerun ends the requeues", n)
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
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	eventually(t, "the task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a run ended by its own context is not a failure to requeue", n)
	}
}

func TestRefusedStartIsNotRequeued(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	tk := &entity.Task{
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the refused task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	close(release)
	tk.Job.Wait()
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a refused start did not fail", n)
	}
}

func TestTaskContextEndsARequeue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("ctx-canceled", &runs),
		Ctx:             ctx,
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	cancel()
	eventually(t, "the task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
	if requeueing(m, tk)() {
		t.Fatal("a task whose context ended is still requeueing")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestTaskContextTakesAQueuedRerunOutOfTheQueue(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	blocker := &entity.Task{Job: job.New("blocker", func(job.Context) error {
		<-block
		return nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("ctx-queued", &runs),
		Ctx:             ctx,
		RequeueAttempts: 1,
		RequeueCooldown: 200 * time.Millisecond,
	}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Add(blocker); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return blocker.Job.IsState(job.StateRunning) })
	eventually(t, "the rerun to be queued", func() bool { return !m.pq.IsEmpty() })
	cancel()
	eventually(t, "the task to stop waiting", func() bool { return !requeueing(m, tk)() })
	if !m.pq.IsEmpty() {
		t.Fatal("a task whose context ended stayed in the queue")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestRemoveEndsARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("removed", &runs),
		RequeueAttempts: 3,
		RequeueCooldown: 50 * time.Millisecond,
	}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Remove(context.Background(), tk); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(tk.ID()); st != nil {
		t.Fatalf("a removed task still has stats: %+v", st)
	}
	eventually(t, "the task to stop waiting", func() bool { return !requeueing(m, tk)() })
	time.Sleep(200 * time.Millisecond) // longer than the cooldown
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a removed task must not run again", n)
	}

	var again atomic.Int32
	next := &entity.Task{Job: failingJob(tk.ID(), 0, &again)}
	if err := m.Add(next); err != nil {
		t.Fatalf("Add under the removed task's ID = %v", err)
	}
	eventually(t, "the new task to run", func() bool { return again.Load() == 1 })
}

func TestRemoveOfAQueuedRerunDoesNotRunIt(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	blocker := &entity.Task{Job: job.New("blocker", func(job.Context) error {
		<-block
		return nil
	})}
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("queued", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: 200 * time.Millisecond,
	}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Add(blocker); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return blocker.Job.IsState(job.StateRunning) })
	eventually(t, "the rerun to be queued", func() bool { return !m.pq.IsEmpty() })
	if err := m.Remove(context.Background(), tk); err != nil {
		t.Fatal(err)
	}
	if !m.pq.IsEmpty() {
		t.Fatal("a removed task stayed in the queue")
	}
	eventually(t, "the task to stop waiting", func() bool { return !requeueing(m, tk)() })
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a removed task must not run again", n)
	}
}

func TestTaskRemovedDuringARequeueNeverRunsAgain(t *testing.T) {
	m := startedManager(t)
	var runs atomic.Int32
	tk := &entity.Task{Job: alwaysFailingJob("requeued", &runs), Manual: true, RequeueAttempts: 100, RequeueCooldown: 10 * time.Millisecond}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a requeue", func() bool { return runs.Load() >= 2 })
	assertRemoved(t, removeInBackground(m, context.Background(), tk))
	n := runs.Load()
	time.Sleep(100 * time.Millisecond)
	if got := runs.Load(); got != n {
		t.Fatalf("runs went from %d to %d after the end was reported", n, got)
	}
}

func TestAddAfterRemoveDuringARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("again", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Remove(context.Background(), tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(tk); err != nil {
		t.Fatalf("Add after Remove = %v", err)
	}
	eventually(t, "the task to run again", func() bool { return runs.Load() == 2 })
	eventually(t, "the new registration to start cooling", requeueing(m, tk))
	time.Sleep(50 * time.Millisecond) // the first requeue must not undo the second as it ends
	if !requeueing(m, tk)() {
		t.Fatal("the removed registration's requeue ended the new one's")
	}
	if st := m.Stats(tk.ID()); st == nil || st.Requeues != 1 {
		t.Fatalf("stats = %+v, want Requeues 1", st)
	}
}

func TestStopEndsARequeue(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("stopped", &runs),
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	stopped := make(chan struct{})
	go func() {
		_ = m.Stop(context.Background())
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop hung on a task waiting out its requeue cooldown")
	}
	if requeueing(m, tk)() {
		t.Fatal("a stopped task is still requeueing")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}
