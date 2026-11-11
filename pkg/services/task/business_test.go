package task

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/cronutil"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

func TestAddRejectsTaskIDAlreadyAdded(t *testing.T) {
	m := startedManager(t)
	first := &entity.Task{Job: job.New("dup", func(job.Context) error { return nil }), Schedule: neverFires}
	second := &entity.Task{Job: job.New("dup", func(job.Context) error { return nil }), Schedule: neverFires}
	if err := m.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(second); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of an ID already added = %v, want Conflict", err)
	}
	if err := m.Add(first); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add of the same task twice = %v, want Conflict", err)
	}
}

func TestAddRejectsAManualTaskWithASchedule(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{Job: countingJob("both", &runs), Manual: true, Schedule: neverFires}
	m := startedManager(t)
	if err := m.Add(tk); !errors.Is(err, errors.InvalidArgument) {
		t.Fatalf("Add = %v, want InvalidArgument", err)
	}
	if m.Stats(tk.ID()) != nil {
		t.Fatal("a rejected task must not stay registered")
	}
}

func TestValidScheduleIsAcceptedByTheManager(t *testing.T) {
	m := newManager()
	for _, spec := range []string{"0 30 6 * * *", "*/5 * * * *", "@daily", "@every 10m"} {
		if err := cronutil.Validate(spec); err != nil {
			t.Fatalf("cronutil.Validate(%q) = %v", spec, err)
		}
		tk := &entity.Task{Job: job.New(spec, func(job.Context) error { return nil }), Ctx: context.Background(), Schedule: spec}
		if err := m.Add(tk); err != nil {
			t.Fatalf("Add rejected %q that validation accepted: %v", spec, err)
		}
	}
}

func removeInBackground(m *manager, ctx context.Context, tasks ...*entity.Task) <-chan error {
	removed := make(chan error, 1)
	go func() { removed <- m.Remove(ctx, tasks...) }()
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

func TestRemoveForgetsTheTask(t *testing.T) {
	m := startedManager(t)
	tk := &entity.Task{Job: job.New("yearly", func(job.Context) error { return nil }), Schedule: neverFires}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if m.Stats(tk.ID()) == nil {
		t.Fatal("an added scheduled task must have stats before it ever runs")
	}
	if err := m.Remove(context.Background(), tk); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(tk.ID()); st != nil {
		t.Fatalf("a removed task still has stats: %+v", st)
	}
}

func TestRemoveOfATaskNotRunningIsDoneAtOnce(t *testing.T) {
	m := startedManager(t)
	var runs atomic.Int32
	tk := &entity.Task{Job: countingJob("idle", &runs), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	assertRemoved(t, removeInBackground(m, context.Background(), tk))
}

func TestRemoveIsDoneOnceTheExecutionHasEnded(t *testing.T) {
	m := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	tk := &entity.Task{Job: holdJob("held", started, release), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	removed := removeInBackground(m, context.Background(), tk)
	assertStillRemoving(t, removed)
	close(release)
	assertRemoved(t, removed)
	if !tk.Job.IsDone() {
		t.Fatalf("job state = %s once Remove returned", tk.Job.State())
	}
}

func TestRemoveOfSeveralIsDoneOnceTheLastHasEnded(t *testing.T) {
	m := startedManager(t)
	firstStarted, firstRelease := make(chan struct{}), make(chan struct{})
	secondStarted, secondRelease := make(chan struct{}), make(chan struct{})
	first := &entity.Task{Job: holdJob("first", firstStarted, firstRelease), Manual: true}
	second := &entity.Task{Job: holdJob("second", secondStarted, secondRelease), Manual: true}
	if err := m.Add(first, second); err != nil {
		t.Fatal(err)
	}
	for _, tk := range []*entity.Task{first, second} {
		if err := m.Trigger(tk.ID(), nil); err != nil {
			t.Fatal(err)
		}
	}
	<-firstStarted
	<-secondStarted
	removed := removeInBackground(m, context.Background(), first, second)
	close(firstRelease)
	assertStillRemoving(t, removed)
	close(secondRelease)
	assertRemoved(t, removed)
}

func TestQueuedTaskRemovedIsDoneAtOnceAndNeverRuns(t *testing.T) {
	m := startedManager(t, MaxConcurrency(1))
	started, release := make(chan struct{}), make(chan struct{})
	blocker := &entity.Task{Job: holdJob("blocker", started, release), Manual: true}
	var runs atomic.Int32
	queued := &entity.Task{Job: countingJob("queued", &runs), Manual: true}
	if err := m.Add(blocker, queued); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(blocker.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := m.Trigger(queued.ID(), nil); err != nil {
		t.Fatal(err)
	}
	assertRemoved(t, removeInBackground(m, context.Background(), queued))
	close(release)
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("the removed task ran %d times", n)
	}
}

func TestRemoveInterruptsARetryWait(t *testing.T) {
	var runs, otherRuns atomic.Int32
	tk := &entity.Task{
		Job:           alwaysFailingJob("remove-retry", &runs),
		RetryAttempts: 3,
		RetryDelay:    time.Hour,
	}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	if err := m.Remove(context.Background(), tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(&entity.Task{Job: countingJob("next", &otherRuns)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be free", func() bool { return otherRuns.Load() == 1 })
}

func TestRemoveKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	m := startedManager(t)
	tk, killed := stubborn(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if err := m.Remove(ctx, tk); !errors.Is(err, context.DeadlineExceeded) {
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
	m := startedManager(t)
	tk, killed := stubborn(t, m)
	if err := m.Remove(canceled(), tk); !errors.Is(err, context.Canceled) {
		t.Fatalf("Remove = %v, want the context's error for what it left", err)
	}
	select {
	case <-killed:
	case <-time.After(5 * time.Second):
		t.Fatal("a forced remove did not kill")
	}
}

func TestRemoveKillsOnlyWhatItRemoves(t *testing.T) {
	m := startedManager(t)
	tk, _ := stubborn(t, m)
	otherStarted, otherRelease := make(chan struct{}), make(chan struct{})
	defer close(otherRelease)
	var otherKilled atomic.Bool
	other := &entity.Task{Job: job.New("other", func(jc job.Context) error {
		close(otherStarted)
		select {
		case <-jc.Signaled(job.SignalKill):
			otherKilled.Store(true)
		case <-otherRelease:
		}
		return nil
	}), Manual: true}
	if err := m.Add(other); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(other.ID(), nil); err != nil {
		t.Fatal(err)
	}
	<-otherStarted
	_ = m.Remove(canceled(), tk)
	time.Sleep(50 * time.Millisecond)
	if otherKilled.Load() {
		t.Fatal("Remove killed a task it did not remove")
	}
}

func TestRemoveOfAStaleTaskLeavesItsSuccessorAlone(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		m := startedManager(t, MaxConcurrency(1))
		var staleRuns, runs atomic.Int32
		stale := &entity.Task{Job: countingJob("same", &staleRuns), Manual: true}
		if err := m.Add(stale); err != nil {
			t.Fatal(err)
		}
		if err := m.Remove(context.Background(), stale); err != nil {
			t.Fatal(err)
		}
		successor := &entity.Task{Job: countingJob("same", &runs), Manual: true}
		started, release := make(chan struct{}), make(chan struct{})
		blocker := &entity.Task{Job: holdJob("blocker", started, release), Manual: true}
		if err := m.Add(successor, blocker); err != nil {
			t.Fatal(err)
		}
		if err := m.Trigger(blocker.ID(), nil); err != nil {
			t.Fatal(err)
		}
		<-started
		if err := m.Trigger(successor.ID(), nil); err != nil {
			t.Fatal(err)
		}
		if err := m.Remove(context.Background(), stale); err != nil {
			t.Fatal(err)
		}
		close(release)
		eventually(t, "the successor to run", func() bool { return runs.Load() == 1 })
	})
	t.Run("scheduled", func(t *testing.T) {
		m := startedManager(t)
		var staleRuns, runs atomic.Int32
		stale := &entity.Task{Job: countingJob("same", &staleRuns), Schedule: neverFires}
		if err := m.Add(stale); err != nil {
			t.Fatal(err)
		}
		if err := m.Remove(context.Background(), stale); err != nil {
			t.Fatal(err)
		}
		successor := &entity.Task{Job: countingJob("same", &runs), Schedule: "* * * * * *"}
		if err := m.Add(successor); err != nil {
			t.Fatal(err)
		}
		if err := m.Remove(context.Background(), stale); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the successor's schedule to run it", func() bool { return runs.Load() >= 1 })
	})
}

// writes its pid to "$0" once it ignores SIGTERM, and so does the child it waits for
const stubbornCommand = `trap "" TERM; sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" & wait`

func waitPid(t *testing.T, path string) int {
	t.Helper()
	var pid int
	eventually(t, "the pid to be written", func() bool {
		raw, err := os.ReadFile(path)
		if err != nil || !strings.HasSuffix(string(raw), "\n") {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		return true
	})
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// an unreaped zombie counts as gone
func gone(pid int) bool {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	rest := string(stat[strings.LastIndexByte(string(stat), ')')+1:])
	return strings.HasPrefix(strings.TrimSpace(rest), "Z")
}

func TestRemoveOfACommandWaitsOutItsGracePeriod(t *testing.T) {
	m := startedManager(t)
	pids := filepath.Join(t.TempDir(), "pid")
	c := jobs.Command{Argv: []string{"sh", "-c", stubbornCommand, pids}, Grace: 200 * time.Millisecond}
	tk := &entity.Task{Job: job.New("stubborn", c.Run), Manual: true}
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	child := waitPid(t, pids)
	begin := time.Now()
	assertRemoved(t, removeInBackground(m, context.Background(), tk))
	if took := time.Since(begin); took < 200*time.Millisecond || took > 2*time.Second {
		t.Fatalf("Remove's end came %s after it, want about the 200ms grace period", took)
	}
	eventually(t, "the child to be gone", func() bool { return gone(child) })
}

func TestManualTaskRunsOnlyWhenTriggered(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{Job: countingJob("manual", &runs), Manual: true}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0: Add must not run a manual task", n)
	}
	for want := int32(1); want <= 2; want++ {
		if err := m.Trigger(tk.ID(), nil); err != nil {
			t.Fatalf("Trigger %d = %v", want, err)
		}
		eventually(t, "the triggered run", func() bool { return runs.Load() == want && !busy(m, tk) })
		if m.Stats(tk.ID()) == nil {
			t.Fatal("a finished manual task must stay registered")
		}
	}
}

func TestTriggerRunsAScheduledTaskNow(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{Job: countingJob("scheduled", &runs), Schedule: neverFires}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the triggered run", func() bool { return runs.Load() == 1 && !busy(m, tk) })
	if m.Stats(tk.ID()) == nil {
		t.Fatal("a scheduled task must stay registered after a triggered run")
	}
}

func TestTriggerOfAnUnknownTaskIsNotFound(t *testing.T) {
	m := startedManager(t)
	if err := m.Trigger("nobody", nil); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Trigger = %v, want NotFound", err)
	}
}

func TestTriggerOfARunningTaskConflicts(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	tk := &entity.Task{Job: blockedJob("running", &runs, release), Manual: true}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := m.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger during a run = %v, want Conflict", err)
	}
}

func TestTriggerOfAQueuedTaskConflicts(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	other := &entity.Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &entity.Task{Job: countingJob("waiting", &runs), Manual: true}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(other, tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return busyRuns.Load() == 1 })
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger while queued = %v, want Conflict", err)
	}
}

func TestTriggerDuringARequeueConflicts(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("cooling-trigger", &runs),
		Manual:          true,
		RequeueAttempts: 1,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Trigger(tk.ID(), nil); !errors.Is(err, errors.Conflict) {
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
	tk := &entity.Task{
		Job: job.New("params", func(jc job.Context) error {
			mu.Lock()
			seen = append(seen, jc.Params())
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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), "first"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the failure and its requeue", func() bool { return runs.Load() == 2 && !busy(m, tk) })
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the next firing", func() bool { return runs.Load() == 3 && !busy(m, tk) })
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
	tk := &entity.Task{Job: blockedJob("cancel-run", &runs, release), Manual: true}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return !busy(m, tk) })
	if st := tk.Job.State(); st != job.StateCanceled {
		t.Fatalf("state = %s, want %s", st, job.StateCanceled)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatalf("Trigger after Cancel = %v", err)
	}
	eventually(t, "the next run", func() bool { return runs.Load() == 2 })
}

func TestCancelEndsAOneShotTask(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	tk := &entity.Task{Job: blockedJob("cancel-one-shot", &runs, release)}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to start", func() bool { return runs.Load() == 1 })
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to be unregistered", func() bool { return m.Stats(tk.ID()) == nil })
}

func TestCancelInterruptsARetryWait(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("cancel-retry", &runs),
		Manual:          true,
		RetryAttempts:   3,
		RetryDelay:      time.Hour,
		RequeueAttempts: 1, // a canceled execution did not fail, so nothing here may run it again
		RequeueCooldown: time.Millisecond,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first run", func() bool { return runs.Load() == 1 })
	eventually(t, "the job to wait for its retry", tk.Job.IsDone)
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the execution to end", func() bool { return !busy(m, tk) })
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCancelEndsARequeueCooldown(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{
		Job:             alwaysFailingJob("cancel-cooling", &runs),
		Manual:          true,
		RequeueAttempts: 3,
		RequeueCooldown: longCooldown,
	}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the task to start cooling", requeueing(m, tk))
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the requeue to end", func() bool { return !busy(m, tk) && !requeueing(m, tk)() })
	if m.Stats(tk.ID()) == nil {
		t.Fatal("a canceled manual task must stay registered")
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestCancelTakesAQueuedTaskOutOfTheQueue(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	other := &entity.Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &entity.Task{Job: countingJob("queued-cancel", &runs), Manual: true}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return busyRuns.Load() == 1 })
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if !m.pq.IsEmpty() {
		t.Fatal("a canceled task stayed in the queue")
	}
	close(release)
	eventually(t, "the slot to be free", func() bool { return other.Job.IsDone() })
	time.Sleep(50 * time.Millisecond)
	if n := runs.Load(); n != 0 {
		t.Fatalf("runs = %d, want 0", n)
	}
	if err := m.Trigger(tk.ID(), nil); err != nil {
		t.Fatalf("Trigger after Cancel = %v", err)
	}
	eventually(t, "the next run", func() bool { return runs.Load() == 1 })
}

func TestCancelOfAQueuedOneShotTaskEndsIt(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	defer close(release)
	other := &entity.Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &entity.Task{Job: countingJob("queued-one-shot", &runs)}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return busyRuns.Load() == 1 })
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(tk.ID()); st != nil {
		t.Fatalf("a canceled one-shot task still has stats: %+v", st)
	}
}

func TestCancelOfATaskWaitingForWhatAForcedStopLeftEndsIt(t *testing.T) {
	m := startedManager(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	tk := &entity.Task{Job: job.New("waiting", func(job.Context) error {
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
	eventually(t, "the task to wait for its last execution", m.pq.IsEmpty)
	if err := m.Cancel(tk.ID()); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(tk.ID()); st != nil {
		t.Fatalf("a canceled one-shot task stayed registered: %+v", st)
	}
	free()
	time.Sleep(100 * time.Millisecond)
	if n := runs.Load(); n != 1 {
		t.Fatalf("runs = %d, want 1: a canceled task ran", n)
	}
}

func TestCancelOfAnIdleTaskConflicts(t *testing.T) {
	var runs atomic.Int32
	tk := &entity.Task{Job: countingJob("idle", &runs), Manual: true}
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(tk.ID()); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Cancel = %v, want Conflict", err)
	}
}

func TestCancelOfAnUnknownTaskIsNotFound(t *testing.T) {
	m := startedManager(t)
	if err := m.Cancel("nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Cancel = %v, want NotFound", err)
	}
}

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
	m := startedManager(t)
	if err := m.Add(tk); err != nil {
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
	if m.Stats(tk.ID()) == nil {
		t.Fatal("the running execution lost its stats to an overlapping tick")
	}
	close(release)
}

func TestTickDuringARunStaysOutOfTheQueue(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, 10)
	tk := &entity.Task{
		Job: job.New("long", func(job.Context) error {
			started <- struct{}{}
			<-release
			return nil
		}),
		Schedule: "* * * * * *",
	}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	<-started
	// every tick during the run finds the task running
	time.Sleep(2200 * time.Millisecond)
	if !m.pq.IsEmpty() {
		t.Fatal("a tick during the run left the task in the queue")
	}
}

func TestTickWhileQueuedIsSkipped(t *testing.T) {
	var busyRuns, runs atomic.Int32
	release := make(chan struct{})
	other := &entity.Task{Job: blockedJob("other", &busyRuns, release)}
	tk := &entity.Task{Job: countingJob("ticking", &runs), Schedule: "* * * * * *"}
	m := startedManager(t, MaxConcurrency(1))
	if err := m.Add(other); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the only slot to be taken", func() bool { return busyRuns.Load() == 1 })
	if err := m.Add(tk); err != nil {
		t.Fatal(err)
	}
	// the first tick waits in the queue; those after it have nowhere to go
	time.Sleep(2200 * time.Millisecond)
	if n := m.pq.Length(); n != 1 {
		t.Fatalf("queue length = %d, want 1", n)
	}
	close(release)
	eventually(t, "the queued tick to run", func() bool { return runs.Load() >= 1 })
}
