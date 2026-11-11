package plan

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/services/task"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

func TestAddRejectsAnInvalidPlan(t *testing.T) {
	cases := []struct {
		name string
		plan *entity.Plan
	}{
		{name: "an unknown kind", plan: &entity.Plan{ID: "x", Kind: "nope", Params: jobs.Command{Argv: []string{"true"}}, Task: &entity.Task{}}},
		{name: "a kind without a task", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand}},
		{name: "params of the wrong type", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Params: "true", Task: &entity.Task{}}},
		{name: "no command", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Params: jobs.Command{}, Task: &entity.Task{}}},
		{name: "params on the task instead of the plan", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Task: &entity.Task{Params: jobs.Command{Argv: []string{"true"}}}}},
		{name: "a command with an output of its own", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Params: jobs.Command{Argv: []string{"true"}, Out: io.Discard}, Task: &entity.Task{}}},
		{name: "params on the task as well", plan: &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Params: jobs.Command{Argv: []string{"true"}}, Task: &entity.Task{Params: "extra"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := startedManager(t)
			if err := m.Add(tc.plan); !errors.Is(err, errors.InvalidArgument) {
				t.Fatalf("Add = %v, want InvalidArgument", err)
			}
			if _, err := m.Trigger("x", entity.PlanRunManual); !errors.Is(err, errors.NotFound) {
				t.Fatalf("a rejected plan stayed: Trigger = %v", err)
			}
		})
	}
}

func TestAddOfAnIDTakenOnTheTaskManagerConflicts(t *testing.T) {
	m, _ := startedManager(t)
	taken := &entity.Task{Job: job.New("taken", func(job.Context) error { return nil }), Manual: true}
	if err := m.tm.Add(taken); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("taken", &entity.Task{}, "true")); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add = %v, want Conflict", err)
	}
}

func TestPlanWithItsOwnJobStillWorks(t *testing.T) {
	m, sender := startedManager(t)
	var runs atomic.Int32
	plan := &entity.Plan{ID: "own", Task: &entity.Task{
		Job:    job.New("own-job", func(job.Context) error { runs.Add(1); return nil }),
		Manual: true,
	}}
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	runID, err := m.Trigger("own", entity.PlanRunManual)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "" {
		t.Fatalf("run ID = %q: a job the plan manager did not build has no run of its own", runID)
	}
	eventually(t, "the run", func() bool { return runs.Load() == 1 })
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events for a plan without a kind: %v", kinds)
	}
}

func TestTriggerAndCancelErrors(t *testing.T) {
	m, sender := startedManager(t)
	if _, err := m.Trigger("nobody", entity.PlanRunManual); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Trigger of an unknown plan = %v, want NotFound", err)
	}
	if err := m.Cancel("nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Cancel of an unknown plan = %v, want NotFound", err)
	}
	if err := m.Add(commandPlan("busy", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel("busy"); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Cancel of an idle plan = %v, want Conflict", err)
	}
	triggered(t, m, "busy")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	if _, err := m.Trigger("busy", entity.PlanRunManual); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Trigger of a busy plan = %v, want Conflict", err)
	}
	if err := m.Cancel("busy"); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerRecordsTheSourceOnTheRunAndItsReruns(t *testing.T) {
	const usage entity.PlanRunSource = "usage"
	cases := []struct {
		name string
		task *entity.Task
		runs int
	}{
		{name: "a single run", task: &entity.Task{}, runs: 1},
		{name: "a retry", task: &entity.Task{RetryAttempts: 1, RetryDelay: time.Millisecond}, runs: 2},
		{name: "a requeue", task: &entity.Task{RequeueAttempts: 1, RequeueCooldown: 10 * time.Millisecond}, runs: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, sender := startedManager(t)
			if err := m.Add(commandPlan("sourced", tc.task, "false")); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Trigger("sourced", usage); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the runs to finish", func() bool { return len(sender.finished()) == tc.runs })
			for _, run := range append(sender.started(), sender.finished()...) {
				if run.Source != usage {
					t.Fatalf("run = %+v, want the source given to Trigger", run)
				}
			}
		})
	}
}

func TestTriggerWithAScheduleSourceIsRecordedAsSuch(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("late", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("late", entity.PlanRunSchedule); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	if src := sender.finished()[0].Source; src != entity.PlanRunSchedule {
		t.Fatalf("source = %s", src)
	}
}

func TestTriggerRejectsAnEmptySource(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("plain", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("plain", ""); !errors.Is(err, errors.InvalidArgument) {
		t.Fatalf("Trigger = %v, want InvalidArgument", err)
	}
	time.Sleep(50 * time.Millisecond)
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events from a rejected Trigger: %v", kinds)
	}
}

func TestCancelOfAFinishedOneShotPlanConflicts(t *testing.T) {
	m, _ := startedManager(t)
	var runs atomic.Int32
	plan := &entity.Plan{ID: "once", Task: &entity.Task{
		Job: job.New("once-job", func(job.Context) error { runs.Add(1); return nil }),
	}}
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run", func() bool { return runs.Load() == 1 && m.tm.Stats("once-job") == nil })
	if err := m.Cancel("once"); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Cancel = %v, want Conflict", err)
	}
}

func TestDeleteOfAnUnknownPlanIsNotFound(t *testing.T) {
	m, _ := startedManager(t)
	if _, err := m.Delete(context.Background(), "nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Delete = %v, want NotFound", err)
	}
}

func TestDeleteOfAnIdlePlanReturnsItsLastRunAtOnce(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("idle", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "idle")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	eventually(t, "the run to settle", func() bool { return !m.plans["idle"].plan.Task.Job.IsExecuting() })
	begin := time.Now()
	run, err := m.Delete(context.Background(), "idle")
	if err != nil {
		t.Fatal(err)
	}
	assertWithin(t, "Delete", 500*time.Millisecond, begin)
	if run == nil || run.ID != runID || run.State != entity.PlanRunSucceeded {
		t.Fatalf("run = %+v", run)
	}
	if _, err := m.Trigger("idle", entity.PlanRunManual); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Trigger after Delete = %v, want NotFound", err)
	}
}

func TestDeleteWithoutARunReturnsNil(t *testing.T) {
	m, _ := startedManager(t)
	if err := m.Add(commandPlan("never", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(ownJobPlan("own", &entity.Task{}, func(job.Context) error { return nil })); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"never", "own"} {
		if run, err := m.Delete(context.Background(), id); err != nil || run != nil {
			t.Fatalf("Delete(%s) = %+v, %v", id, run, err)
		}
	}
}

func TestDeleteReturnsOnlyOnceTheCanceledRunHasEnded(t *testing.T) {
	m, sender := startedManager(t)
	ready := filepath.Join(t.TempDir(), "ready")
	script := `trap 'sleep 0.3; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("slow", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "slow")
	eventually(t, "the command to get ready", func() bool { _, err := os.Stat(ready); return err == nil })
	begin := time.Now()
	run, err := m.Delete(context.Background(), "slow")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(begin); took < 300*time.Millisecond {
		t.Fatalf("Delete returned after %s, before the command finished cleaning up", took)
	}
	if run == nil || run.ID != runID || run.State != entity.PlanRunCanceled || run.EndedAt.IsZero() {
		t.Fatalf("run = %+v", run)
	}
	if finished := sender.finished(); len(finished) != 1 || finished[0].State != entity.PlanRunCanceled {
		t.Fatalf("finished = %+v", finished)
	}
}

func TestDeleteOfAQueuedPlanMeansItNeverRuns(t *testing.T) {
	m, sender := startedManagerOn(t, startedTaskManager(t, task.MaxConcurrency(1)))
	if err := m.Add(commandPlan("blocker", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("queued", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "blocker")
	eventually(t, "the blocker to start", func() bool { return len(sender.started()) == 1 })
	triggered(t, m, "queued")
	if run, err := m.Delete(context.Background(), "queued"); err != nil || run != nil {
		t.Fatalf("Delete = %+v, %v", run, err)
	}
	if err := m.Cancel("blocker"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the blocker to end", func() bool { return len(sender.finished()) == 1 })
	time.Sleep(50 * time.Millisecond)
	for _, run := range sender.started() {
		if run.PlanID == "queued" {
			t.Fatalf("the deleted plan ran: %+v", run)
		}
	}
}

func TestDeleteDuringARequeueEndsIt(t *testing.T) {
	m, sender := startedManager(t)
	plan := commandPlan("requeued", &entity.Task{RequeueAttempts: 100, RequeueCooldown: 10 * time.Millisecond}, "false")
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "requeued")
	eventually(t, "a requeue", func() bool { return len(sender.finished()) >= 2 })
	run, err := m.Delete(context.Background(), "requeued")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.PlanID != "requeued" {
		t.Fatalf("run = %+v", run)
	}
	finished := len(sender.finished())
	if started := len(sender.started()); started != finished {
		t.Fatalf("%d runs started and %d finished when Delete returned", started, finished)
	}
	time.Sleep(100 * time.Millisecond)
	if got := len(sender.started()); got != finished {
		t.Fatalf("runs went from %d to %d after Delete", finished, got)
	}
}

func TestNoCronRunStartsAfterDeleteBegan(t *testing.T) {
	m, sender := startedManager(t)
	ready := filepath.Join(t.TempDir(), "ready")
	// the run ends just as the next ticks are due
	script := `trap 'sleep 1.1; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("cron", &entity.Task{Schedule: "* * * * * *"}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a scheduled run", func() bool { _, err := os.Stat(ready); return err == nil })
	if _, err := m.Delete(context.Background(), "cron"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if started := sender.started(); len(started) != 1 {
		t.Fatalf("started = %+v, want the one run that was going", started)
	}
}

func TestDeleteReturnsTheRunStillGoingOnceItsContextEnds(t *testing.T) {
	m, _ := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(withGrace(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids), time.Minute)); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	run, err := m.Delete(ctx, "stubborn")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Delete = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took < 200*time.Millisecond {
		t.Fatalf("Delete gave up after %s, before its deadline", took)
	}
	if run == nil || run.ID != runID {
		t.Fatalf("run = %+v", run)
	}
	assertGone(t, child)
}

// forcedDelete deletes with an ended context, which leaves the run to end as the kill lands.
func forcedDelete(t *testing.T, m *manager, id string) *entity.PlanRun {
	t.Helper()
	begin := time.Now()
	run, err := m.Delete(canceled(), id)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("forced Delete = %v, want the context's error for the run it left", err)
	}
	assertWithin(t, "a forced Delete", time.Second, begin)
	return run
}

func TestAForcedDeleteKillsACommandIgnoringTheSignalAtOnce(t *testing.T) {
	m, sender := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(withGrace(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids), time.Minute)); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	if run := forcedDelete(t, m, "stubborn"); run == nil || run.ID != runID {
		t.Fatalf("run = %+v", run)
	}
	assertGone(t, child)
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	if st := sender.finished()[0].State; st != entity.PlanRunCanceled {
		t.Fatalf("state = %s", st)
	}
}

func TestAForcedDeleteCutsTheGracePeriodOfACanceledRunShort(t *testing.T) {
	m, _ := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(withGrace(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids), time.Minute)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	if err := m.Cancel("stubborn"); err != nil {
		t.Fatal(err)
	}
	forcedDelete(t, m, "stubborn")
	assertGone(t, child)
}

func TestDeleteOfAnOwnJobHonoringTheCancelReturnsAsItEnds(t *testing.T) {
	m, _ := startedManager(t)
	var started atomic.Bool
	plan := ownJobPlan("own", &entity.Task{}, func(jc job.Context) error {
		started.Store(true)
		<-jc.Context().Done()
		return nil
	})
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("own", entity.PlanRunManual); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job to start", started.Load)
	begin := time.Now()
	if _, err := m.Delete(context.Background(), "own"); err != nil {
		t.Fatal(err)
	}
	assertWithin(t, "Delete", time.Second, begin)
}

func TestDeleteOfAnOwnJobIgnoringTheCancelReturnsOnceItsContextEnds(t *testing.T) {
	cases := []struct {
		name  string
		ctx   func() (context.Context, context.CancelFunc)
		least time.Duration
		most  time.Duration
	}{
		{name: "an ended context returns at once", ctx: func() (context.Context, context.CancelFunc) { return canceled(), func() {} }, most: time.Second},
		{name: "a deadline returns as it passes", ctx: func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 200*time.Millisecond)
		}, least: 200 * time.Millisecond, most: 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := startedManager(t)
			release := make(chan struct{})
			defer close(release)
			var started atomic.Bool
			plan := ownJobPlan("own", &entity.Task{}, func(job.Context) error {
				started.Store(true)
				<-release
				return nil
			})
			if err := m.Add(plan); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Trigger("own", entity.PlanRunManual); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the job to start", started.Load)
			ctx, cancel := tc.ctx()
			defer cancel()
			begin := time.Now()
			run, err := m.Delete(ctx, "own")
			if !errors.Is(err, ctx.Err()) {
				t.Fatalf("Delete = %v, want the context's error", err)
			}
			if took := time.Since(begin); took < tc.least || took > tc.most {
				t.Fatalf("Delete took %s, want %s to %s", took, tc.least, tc.most)
			}
			if run != nil {
				t.Fatalf("run = %+v: a plan without a kind has none", run)
			}
		})
	}
}

func TestDeleteSendsTheKillToAnOwnJobOnceItsContextEnds(t *testing.T) {
	m, _ := startedManager(t)
	var started atomic.Bool
	killed := make(chan time.Time, 1)
	plan := ownJobPlan("own", &entity.Task{}, func(jc job.Context) error {
		started.Store(true)
		<-jc.Signaled(job.SignalKill)
		killed <- time.Now()
		return nil
	})
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("own", entity.PlanRunManual); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job to start", started.Load)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if _, err := m.Delete(ctx, "own"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Delete = %v, want the deadline's error", err)
	}
	select {
	case at := <-killed:
		if took := at.Sub(begin); took < 200*time.Millisecond || took > 2*time.Second {
			t.Fatalf("the kill came %s after Delete, want as its 200ms deadline passes", took)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the deadline passed without a kill")
	}
}

func TestAForcedDeleteSendsTheKillToAnOwnJob(t *testing.T) {
	m, _ := startedManager(t)
	var started, killed atomic.Bool
	plan := ownJobPlan("own", &entity.Task{}, func(jc job.Context) error {
		started.Store(true)
		<-jc.Signaled(job.SignalKill)
		killed.Store(true)
		return nil
	})
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("own", entity.PlanRunManual); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job to start", started.Load)
	forcedDelete(t, m, "own")
	eventually(t, "the job to see the kill", killed.Load)
}

func TestARunLeftByADeleteStaysOffThePlanAddedAgain(t *testing.T) {
	m, sender := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(withGrace(commandPlan("same", &entity.Task{}, "sh", "-c", stubbornCommand, pids), time.Minute)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "same")
	waitPid(t, pids)
	forcedDelete(t, m, "same")
	if err := m.Add(commandPlan("same", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run left behind to end", func() bool { return len(sender.finished()) == 1 })
	if run, err := m.LastRun("same"); err != nil || run != nil {
		t.Fatalf("LastRun = %+v, %v: the plan added again has not run", run, err)
	}
}

func TestDeleteUnderStress(t *testing.T) {
	m, sender := startedManagerOn(t, startedTaskManager(t, task.MaxConcurrency(4)))
	tasks := []func() *entity.Task{
		func() *entity.Task { return &entity.Task{} },
		func() *entity.Task { return &entity.Task{RetryAttempts: 2, RetryDelay: time.Millisecond} },
		func() *entity.Task { return &entity.Task{RequeueAttempts: 3, RequeueCooldown: time.Millisecond} },
		func() *entity.Task { return &entity.Task{Schedule: "* * * * * *"} },
		func() *entity.Task { return &entity.Task{Timeout: time.Millisecond} },
	}
	commands := [][]string{{"true"}, {"false"}, {"sleep", "0.01"}, {"sleep", "30"}}
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(worker)))
			id := fmt.Sprintf("plan-%d", worker)
			for range 40 {
				plan := commandPlan(id, tasks[r.Intn(len(tasks))](), commands[r.Intn(len(commands))]...)
				if err := m.Add(plan); err != nil {
					t.Errorf("Add = %v", err)
					return
				}
				for range r.Intn(3) {
					_, _ = m.Trigger(id, entity.PlanRunManual)
					time.Sleep(time.Duration(r.Intn(2000)) * time.Microsecond)
				}
				if r.Intn(3) == 0 {
					_ = m.Cancel(id)
				}
				ctx, forced := context.Background(), r.Intn(2) == 0
				if forced {
					ctx = canceled()
				}
				if _, err := m.Delete(ctx, id); err != nil && !(forced && errors.Is(err, context.Canceled)) {
					t.Errorf("Delete = %v", err)
					return
				}
				if forced {
					// a forced Delete returns before its run ends, maybe before it reports its start
					_ = m.tm.Remove(context.Background(), plan.Task)
				}
				if started, finished := sender.counts(id); started != finished {
					t.Errorf("%d runs started and %d finished after Delete", started, finished)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestResultOfACommandPlanIsNil(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("ran", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "ran")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	if res, err := m.Result("ran"); err != nil || res != nil {
		t.Fatalf("Result = %+v, %v: a command plan's job keeps no result, its runs are LastRun's", res, err)
	}
}

func TestLastRunOfAnUnknownPlanIsNotFound(t *testing.T) {
	m, _ := startedManager(t)
	if _, err := m.LastRun("nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("LastRun = %v, want NotFound", err)
	}
}

func TestLastRunFollowsTheLatestRun(t *testing.T) {
	m, sender := startedManager(t)
	release := filepath.Join(t.TempDir(), "release")
	script := "while [ ! -e " + release + " ]; do sleep 0.01; done"
	if err := m.Add(commandPlan("latest", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	if run, err := m.LastRun("latest"); err != nil || run != nil {
		t.Fatalf("LastRun before any run = %+v, %v", run, err)
	}
	runID := triggered(t, m, "latest")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	run, err := m.LastRun("latest")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.ID != runID || run.State != entity.PlanRunRunning {
		t.Fatalf("LastRun during the run = %+v", run)
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	eventually(t, "the run's end to show", func() bool {
		run, err := m.LastRun("latest")
		return err == nil && run != nil && run.ID == runID && run.State == entity.PlanRunSucceeded
	})
}

func TestLastRunOfAPlanWithoutAKindIsNil(t *testing.T) {
	m, _ := startedManager(t)
	var runs atomic.Int32
	plan := &entity.Plan{ID: "own", Task: &entity.Task{
		Job: job.New("own-job", func(jc job.Context) error {
			jc.SetResult(entity.PlanRun{ID: "not a run of the plan manager's"})
			runs.Add(1)
			return nil
		}),
		Manual: true,
	}}
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Trigger("own", entity.PlanRunManual); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job", func() bool { return runs.Load() == 1 })
	if run, err := m.LastRun("own"); err != nil || run != nil {
		t.Fatalf("LastRun = %+v, %v", run, err)
	}
}

func writeLogFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenRunLogReadsTheFile(t *testing.T) {
	cases := []struct {
		name string
		tail int64
		want string
	}{
		{name: "the whole file", tail: 0, want: "0123456789"},
		{name: "its tail", tail: 4, want: "6789"},
		{name: "a tail longer than the file", tail: 100, want: "0123456789"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			m := newManager(nil, task.New(), WithLogDir(dir))
			path := writeLogFile(t, dir, "plan/run.log", "0123456789")
			f, err := m.OpenRunLog(path, tc.tail)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			raw, err := io.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != tc.want {
				t.Fatalf("read %q, want %q", raw, tc.want)
			}
		})
	}
}

func TestOpenRunLogTakesAFileStillBeingWritten(t *testing.T) {
	dir := t.TempDir()
	m := newManager(nil, task.New(), WithLogDir(dir))
	path := writeLogFile(t, dir, "plan/run.log", "first\n")
	w, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	f, err := m.OpenRunLog(path, 6)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := w.WriteString("second\n"); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "first\nsecond\n" {
		t.Fatalf("read %q", raw)
	}
}

func TestOpenRunLogOfARealRun(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("logged", &entity.Task{}, "sh", "-c", "echo hello")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "logged")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	f, err := m.OpenRunLog(sender.finished()[0].LogPath, 6)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "hello\n" {
		t.Fatalf("read %q", raw)
	}
}

func TestOpenRunLogKeepsToTheLogDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs")
	m := newManager(nil, task.New(), WithLogDir(dir))
	writeLogFile(t, parent, "logs-other/run.log", "secret")
	writeLogFile(t, parent, "outside.log", "secret")
	writeLogFile(t, dir, "plan/run.log", "")
	cases := []struct {
		name string
		path string
	}{
		{name: "an unrelated absolute path", path: "/etc/passwd"},
		{name: "a path climbing out with ..", path: filepath.Join(dir, "..", "outside.log")},
		{name: "a raw path climbing out with ..", path: dir + "/plan/../../outside.log"},
		{name: "a sibling sharing the prefix", path: filepath.Join(parent, "logs-other", "run.log")},
		{name: "the log dir itself", path: dir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := m.OpenRunLog(tc.path, 0); !errors.Is(err, errors.Forbidden) {
				t.Fatalf("OpenRunLog = %v, want Forbidden", err)
			}
		})
	}
}

func TestOpenRunLogTakesARelativeLogDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	m := newManager(nil, task.New(), WithLogDir("logs"))
	writeLogFile(t, dir, "logs/plan/run.log", "hi")
	f, err := m.OpenRunLog(filepath.Join("logs", "plan", "run.log"), 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := m.OpenRunLog(filepath.Join("..", "run.log"), 0); !errors.Is(err, errors.Forbidden) {
		t.Fatalf("OpenRunLog = %v, want Forbidden", err)
	}
}

func TestOpenRunLogNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := newManager(nil, task.New(), WithLogDir(dir)).OpenRunLog(filepath.Join(dir, "gone.log"), 0); !errors.Is(err, errors.NotFound) {
		t.Fatalf("a missing file = %v, want NotFound", err)
	}
	if _, err := newManager(nil, task.New()).OpenRunLog(filepath.Join(dir, "any.log"), 0); !errors.Is(err, errors.NotFound) {
		t.Fatalf("no log dir = %v, want NotFound", err)
	}
}
