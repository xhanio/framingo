package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/job"
)

// a schedule that never fires during a test
const neverFires = "0 0 1 1 *"

func countingJob(id string, runs *atomic.Int32) job.Job {
	return job.New(id, func(job.Context) error {
		runs.Add(1)
		return nil
	})
}

// blockedJob counts its runs and holds each one until release closes or it is canceled.
func blockedJob(id string, runs *atomic.Int32, release <-chan struct{}) job.Job {
	return job.New(id, func(jc job.Context) error {
		runs.Add(1)
		select {
		case <-release:
			return nil
		case <-jc.Context().Done():
			return jc.Context().Err()
		}
	})
}

func TestManualTaskRunsOnlyWhenTriggered(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{Job: countingJob("manual", &runs), Manual: true}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0: Add must not run a manual task", n)
	}
	for want := int32(1); want <= 2; want++ {
		if err := s.Trigger(tk.ID(), nil); err != nil {
			t.Fatalf("Trigger %d = %v", want, err)
		}
		eventually(t, "the triggered run", func() bool { return runs.Load() == want && !busy(s, tk) })
		if s.Stats(tk.ID()) == nil {
			t.Fatal("a finished manual task must stay registered")
		}
	}
}

func requeueing(s *manager, tk *Task) func() bool {
	return func() bool {
		st := s.Stats(tk.ID())
		return st != nil && st.Requeueing
	}
}

func registrationOf(s *manager, tk *Task) *registration {
	s.el.RLock()
	defer s.el.RUnlock()
	return s.registered[tk.ID()]
}

// busy reports whether a task is queued, running or being requeued.
func busy(s *manager, tk *Task) bool {
	s.el.RLock()
	defer s.el.RUnlock()
	_, running := s.running[tk]
	reg := s.registered[tk.ID()]
	return running || (reg != nil && (reg.queued || reg.cooldown != nil))
}

func TestAddRejectsAManualTaskWithASchedule(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{Job: countingJob("both", &runs), Manual: true, Schedule: neverFires}
	s := startedScheduler(t)
	if err := s.Add(tk); !errors.Is(err, errors.InvalidArgument) {
		t.Fatalf("Add = %v, want InvalidArgument", err)
	}
	if s.Stats(tk.ID()) != nil {
		t.Fatal("a rejected task must not stay registered")
	}
}

func TestTriggerRunsAScheduledTaskNow(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{Job: countingJob("scheduled", &runs), Schedule: neverFires}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the triggered run", func() bool { return runs.Load() == 1 && !busy(s, tk) })
	if s.Stats(tk.ID()) == nil {
		t.Fatal("a scheduled task must stay registered after a triggered run")
	}
}

func TestTriggerOfAnUnknownTaskIsNotFound(t *testing.T) {
	s := startedScheduler(t)
	if err := s.Trigger("nobody", nil); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Trigger = %v, want NotFound", err)
	}
}

func TestTriggerOfARunningTaskConflicts(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	tk := &Task{Job: blockedJob("running", &runs, release), Manual: true}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := s.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger during a run = %v, want Conflict", err)
	}
}

func TestTriggerOfAQueuedTaskConflicts(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	other := &Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &Task{Job: countingJob("waiting", &runs), Manual: true}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(other, tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busyRuns.Load() == 1 })
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger while queued = %v, want Conflict", err)
	}
}

func TestTriggerDuringARequeueConflicts(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("cooling-trigger", &runs),
		Manual:          true,
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(s, tk))
	if err := s.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger during a requeue = %v, want Conflict", err)
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestTriggerParamsLastForThatFiring(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []any
	)
	var runs atomic.Int32
	tk := &Task{
		Job: job.New("params", func(jc job.Context) error {
			mu.Lock()
			seen = append(seen, jc.GetParams())
			mu.Unlock()
			if runs.Add(1) == 1 {
				return errors.Newf("run failed")
			}
			return nil
		}),
		Params:          "default",
		Manual:          true,
		RequeueAttempts: 1,
		RequeueCooldown: 10 * time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), "first"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the failure and its requeue", func() bool { return runs.Load() == 2 && !busy(s, tk) })
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the next firing", func() bool { return runs.Load() == 3 && !busy(s, tk) })
	mu.Lock()
	defer mu.Unlock()
	want := []any{"first", "first", "default"}
	if len(seen) != len(want) {
		t.Fatalf("params seen = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("params seen = %v, want %v", seen, want)
		}
	}
}

func TestCancelOfARunEndsItAndKeepsTheTask(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	tk := &Task{Job: blockedJob("cancel-run", &runs, release), Manual: true}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return !busy(s, tk) })
	if st := tk.Job.State(); st != job.StateCanceled {
		t.Fatalf("state = %s, want %s", st, job.StateCanceled)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatalf("Trigger after Cancel = %v", err)
	}
	eventually(t, "the next run", func() bool { return runs.Load() == 2 })
}

func TestCancelEndsAOneShotTask(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	tk := &Task{Job: blockedJob("cancel-one-shot", &runs, release)}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to be unregistered", func() bool { return s.Stats(tk.ID()) == nil })
}

func TestCancelInterruptsARetryWait(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("cancel-retry", &runs),
		Manual:          true,
		RetryAttempts:   3,
		RetryDelay:      time.Hour,
		RequeueAttempts: 1, // a canceled execution did not fail, so nothing here may run it again
		RequeueCooldown: time.Millisecond,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the execution to end", func() bool { return !busy(s, tk) })
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCancelEndsARequeueCooldown(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:             alwaysFailingJob("cancel-cooling", &runs),
		Manual:          true,
		RequeueAttempts: 3,
		RequeueCooldown: longCooldown,
	}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(s, tk))
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeue to end", func() bool { return !busy(s, tk) && !requeueing(s, tk)() })
	if s.Stats(tk.ID()) == nil {
		t.Fatal("a canceled manual task must stay registered")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCancelTakesAQueuedTaskOutOfTheQueue(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	other := &Task{Job: blockedJob("other", &busyRuns, release)}
	// the fetching goroutine holds this one at the handoff, leaving tk in the queue
	held := &Task{Job: job.New("held", func(job.Context) error { return nil }), Priority: -1}
	tk := &Task{Job: countingJob("queued-cancel", &runs), Manual: true}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busyRuns.Load() == 1 })
	if err := s.Add(held, tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fetching goroutine to hold a task", s.pq.IsEmpty)
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if !s.pq.IsEmpty() {
		t.Fatal("a canceled task stayed in the queue")
	}
	close(release)
	eventually(t, "the worker to be free", func() bool { return held.Job.IsDone() })
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0", n)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatalf("Trigger after Cancel = %v", err)
	}
	eventually(t, "the next run", func() bool { return runs.Load() == 1 })
}

func TestCancelOfAQueuedOneShotTaskEndsIt(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	other := &Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &Task{Job: countingJob("queued-one-shot", &runs)}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busyRuns.Load() == 1 })
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(tk.ID()); st != nil {
		t.Fatalf("a canceled one-shot task still has stats: %+v", st)
	}
}

func TestCancelOfAnIdleTaskConflicts(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{Job: countingJob("idle", &runs), Manual: true}
	s := startedScheduler(t)
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Cancel(tk.ID()); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Cancel = %v, want Conflict", err)
	}
}

func TestCancelOfAnUnknownTaskIsNotFound(t *testing.T) {
	s := startedScheduler(t)
	if err := s.Cancel("nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Cancel = %v, want NotFound", err)
	}
}

func TestRemoveInterruptsARetryWait(t *testing.T) {
	var runs, otherRuns atomic.Int32
	tk := &Task{
		Job:           alwaysFailingJob("remove-retry", &runs),
		RetryAttempts: 3,
		RetryDelay:    time.Hour,
	}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	s.Remove(tk)
	if err := s.Add(&Task{Job: countingJob("next", &otherRuns)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be free", func() bool { return otherRuns.Load() == 1 })
}

func TestStopInterruptsARetryWait(t *testing.T) {
	var runs atomic.Int32
	tk := &Task{
		Job:           alwaysFailingJob("stop-retry", &runs),
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
	s := newScheduler(MaxConcurrency(1))
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	if err := s.Stop(true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond) // past the retry the stop must have ended
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a stopped manager must not retry", n)
	}
}

func TestTickWhileQueuedIsSkipped(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	other := &Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &Task{Job: countingJob("ticking", &runs), Schedule: "* * * * * *"}
	s := startedScheduler(t, MaxConcurrency(1))
	if err := s.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only worker to be busy", func() bool { return busyRuns.Load() == 1 })
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	// the first tick is held at the handoff; those after it have nowhere to go
	time.Sleep(2200 * time.Millisecond)
	if !s.pq.IsEmpty() {
		t.Fatal("a tick behind a queued task stayed in the queue")
	}
	close(release)
}
