package planner

import (
	"context"
	"fmt"
	"math/rand"
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
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/task"
)

// each writes its pid once it ignores SIGTERM, so a test can signal it from then on
const (
	// the shell dies of SIGTERM and the child does not
	stubbornChild = `sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" & wait`
	// neither does
	stubbornCommand = `trap "" TERM; ` + stubbornChild
)

func pidFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "pid")
}

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

func assertGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if gone(pid) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("process %d outlived the run", pid)
}

func assertWithin(t *testing.T, what string, limit time.Duration, begin time.Time) {
	t.Helper()
	if took := time.Since(begin); took > limit {
		t.Fatalf("%s took %s, want at most %s", what, took, limit)
	}
}

func ownJobPlan(id string, tk *task.Task, fn job.Func) *entity.Plan {
	tk.Job = job.New(id+"-job", fn)
	tk.Manual = true
	return &entity.Plan{ID: id, Task: tk}
}

func TestDeleteOfAnUnknownPlanIsNotFound(t *testing.T) {
	m, _ := startedPlanner(t)
	if _, err := m.Delete("nobody", false); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Delete = %v, want NotFound", err)
	}
}

func TestDeleteOfAnIdlePlanReturnsItsLastRunAtOnce(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("idle", &task.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "idle")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	eventually(t, "the run to settle", func() bool { return !m.plans["idle"].Task.Job.IsExecuting() })
	begin := time.Now()
	run, err := m.Delete("idle", false)
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
	m, _ := startedPlanner(t)
	if err := m.Add(commandPlan("never", &task.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(ownJobPlan("own", &task.Task{}, func(job.Context) error { return nil })); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"never", "own"} {
		if run, err := m.Delete(id, false); err != nil || run != nil {
			t.Fatalf("Delete(%s) = %+v, %v", id, run, err)
		}
	}
}

func TestDeleteReturnsOnlyOnceTheCanceledRunHasEnded(t *testing.T) {
	m, sender := startedPlanner(t)
	ready := filepath.Join(t.TempDir(), "ready")
	script := `trap 'sleep 0.3; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("slow", &task.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "slow")
	eventually(t, "the command to get ready", func() bool { _, err := os.Stat(ready); return err == nil })
	begin := time.Now()
	run, err := m.Delete("slow", false)
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
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t, task.MaxConcurrency(1)), WithLogDir(t.TempDir()))
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	if err := m.Add(commandPlan("blocker", &task.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("queued", &task.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "blocker")
	eventually(t, "the blocker to start", func() bool { return len(sender.started()) == 1 })
	triggered(t, m, "queued")
	if run, err := m.Delete("queued", false); err != nil || run != nil {
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
	m, sender := startedPlanner(t)
	plan := commandPlan("requeued", &task.Task{RequeueAttempts: 100, RequeueCooldown: 10 * time.Millisecond}, "false")
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "requeued")
	eventually(t, "a requeue", func() bool { return len(sender.finished()) >= 2 })
	run, err := m.Delete("requeued", false)
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
	m, sender := startedPlanner(t)
	ready := filepath.Join(t.TempDir(), "ready")
	// the run ends just as the next ticks are due
	script := `trap 'sleep 1.1; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("cron", &task.Task{Schedule: "* * * * * *"}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a scheduled run", func() bool { _, err := os.Stat(ready); return err == nil })
	if _, err := m.Delete("cron", false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if started := sender.started(); len(started) != 1 {
		t.Fatalf("started = %+v, want the one run that was going", started)
	}
}

func TestDeleteWithForceEndsACommandIgnoringTheSignalAtOnce(t *testing.T) {
	m, _ := startedPlanner(t)
	pids := pidFile(t)
	if err := m.Add(commandPlan("stubborn", &task.Task{}, "sh", "-c", stubbornCommand, pids)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	begin := time.Now()
	run, err := m.Delete("stubborn", true)
	if err != nil {
		t.Fatal(err)
	}
	assertWithin(t, "Delete with force", time.Second, begin)
	if run == nil || run.State != entity.PlanRunCanceled {
		t.Fatalf("run = %+v", run)
	}
	assertGone(t, child)
}

func TestDeleteWithForceCutsTheGracePeriodOfACanceledRunShort(t *testing.T) {
	m, _ := startedPlanner(t)
	pids := pidFile(t)
	if err := m.Add(commandPlan("stubborn", &task.Task{}, "sh", "-c", stubbornCommand, pids)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	if err := m.Cancel("stubborn"); err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	run, err := m.Delete("stubborn", true)
	if err != nil {
		t.Fatal(err)
	}
	assertWithin(t, "Delete with force", time.Second, begin)
	if run == nil || run.State != entity.PlanRunCanceled {
		t.Fatalf("run = %+v", run)
	}
	assertGone(t, child)
}

func TestDeleteOfAnOwnJobIgnoringTheCancel(t *testing.T) {
	cases := []struct {
		name  string
		force bool
		grace time.Duration
		least time.Duration
		most  time.Duration
	}{
		{name: "with force returns at once", force: true, grace: time.Minute, most: time.Second},
		{name: "without force returns as the grace period ends", grace: 200 * time.Millisecond, least: 200 * time.Millisecond, most: 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := startedPlanner(t)
			release := make(chan struct{})
			defer close(release)
			var started atomic.Bool
			plan := ownJobPlan("own", &task.Task{GracePeriod: tc.grace}, func(job.Context) error {
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
			begin := time.Now()
			run, err := m.Delete("own", tc.force)
			if err != nil {
				t.Fatal(err)
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

func TestDeleteOfAnOwnJobHonoringTheCancelDoesNotWaitOutTheGracePeriod(t *testing.T) {
	m, _ := startedPlanner(t)
	var started atomic.Bool
	plan := ownJobPlan("own", &task.Task{}, func(jc job.Context) error {
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
	if _, err := m.Delete("own", false); err != nil {
		t.Fatal(err)
	}
	assertWithin(t, "Delete", time.Second, begin)
}

func TestNoProcessOutlivesAStoppedRun(t *testing.T) {
	const grace = 200 * time.Millisecond
	stops := []struct {
		name  string
		task  func() *task.Task
		stop  func(*testing.T, *manager)
		state entity.PlanRunState
	}{
		{
			name: "Cancel", task: func() *task.Task { return &task.Task{GracePeriod: grace} }, state: entity.PlanRunCanceled,
			stop: func(t *testing.T, m *manager) {
				if err := m.Cancel("stubborn"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Delete", task: func() *task.Task { return &task.Task{GracePeriod: grace} }, state: entity.PlanRunCanceled,
			stop: func(t *testing.T, m *manager) {
				if _, err := m.Delete("stubborn", false); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a timeout", task: func() *task.Task { return &task.Task{GracePeriod: grace, Timeout: 300 * time.Millisecond} }, state: entity.PlanRunFailed,
			stop: func(*testing.T, *manager) {},
		},
	}
	scripts := []struct {
		name   string
		script string
	}{
		{name: "a stubborn child", script: stubbornChild},
		{name: "a stubborn child without the output", script: `sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" >/dev/null 2>&1 & wait`},
		{name: "a stubborn command", script: stubbornCommand},
	}
	for _, sc := range scripts {
		for _, tc := range stops {
			t.Run(sc.name+"/"+tc.name, func(t *testing.T) {
				m, sender := startedPlanner(t)
				pids := pidFile(t)
				if err := m.Add(commandPlan("stubborn", tc.task(), "sh", "-c", sc.script, pids)); err != nil {
					t.Fatal(err)
				}
				triggered(t, m, "stubborn")
				child := waitPid(t, pids)
				tc.stop(t, m)
				eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
				if st := sender.finished()[0].State; st != tc.state {
					t.Fatalf("state = %s, want %s", st, tc.state)
				}
				assertGone(t, child)
			})
		}
	}
}

func TestShortGracePeriodEndsEarlierThanTheDefault(t *testing.T) {
	m, sender := startedPlanner(t)
	pids := pidFile(t)
	plan := commandPlan("stubborn", &task.Task{GracePeriod: 100 * time.Millisecond}, "sh", "-c", stubbornCommand, pids)
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	waitPid(t, pids)
	begin := time.Now()
	if err := m.Cancel("stubborn"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	assertWithin(t, "the canceled run", 2*time.Second, begin)
}

func TestZeroGracePeriodStillGivesACommandTimeToCleanUp(t *testing.T) {
	m, sender := startedPlanner(t)
	ready := filepath.Join(t.TempDir(), "ready")
	script := `trap 'sleep 0.3; echo cleaned; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("tidy", &task.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "tidy")
	eventually(t, "the command to get ready", func() bool { _, err := os.Stat(ready); return err == nil })
	if err := m.Cancel("tidy"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	if tail := sender.finished()[0].OutputTail; tail != "cleaned\n" {
		t.Fatalf("output = %q: the command was cut off while cleaning up", tail)
	}
}

func TestCommandEndingOnItsOwnKeepsItsBackgroundChild(t *testing.T) {
	m, sender := startedPlanner(t)
	pids := pidFile(t)
	script := `sleep 60 >/dev/null 2>&1 & echo $! > "$0"`
	if err := m.Add(commandPlan("daemonizer", &task.Task{}, "sh", "-c", script, pids)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "daemonizer")
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	if st := sender.finished()[0].State; st != entity.PlanRunSucceeded {
		t.Fatalf("state = %s", st)
	}
	child := waitPid(t, pids)
	time.Sleep(50 * time.Millisecond)
	if gone(child) {
		t.Fatal("a command that ended on its own took its background child with it")
	}
}

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestAForcedPlannerStopKills(t *testing.T) {
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t), WithLogDir(t.TempDir()))
	pids := pidFile(t)
	if err := m.Add(commandPlan("stubborn", &task.Task{}, "sh", "-c", stubbornCommand, pids)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	begin := time.Now()
	if err := m.Stop(canceled()); !errors.Is(err, context.Canceled) {
		t.Fatalf("forced stop = %v, want the context's error for the run it left", err)
	}
	assertWithin(t, "a forced stop", time.Second, begin)
	assertGone(t, child)
}

func TestPlannerStopKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t), WithLogDir(t.TempDir()))
	pids := pidFile(t)
	if err := m.Add(commandPlan("stubborn", &task.Task{GracePeriod: time.Minute}, "sh", "-c", stubbornCommand, pids)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "stubborn")
	child := waitPid(t, pids)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	if err := m.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop = %v, want the deadline's error", err)
	}
	if took := time.Since(begin); took < 200*time.Millisecond {
		t.Fatalf("Stop gave up after %s, before its deadline", took)
	}
	assertWithin(t, "Stop past its deadline", 2*time.Second, begin)
	assertGone(t, child)
}

func TestPlannerStopWaitsForTheSlowestPlanNotTheSum(t *testing.T) {
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t), WithLogDir(t.TempDir()))
	dir := t.TempDir()
	const plans = 3
	for i := range plans {
		id := fmt.Sprintf("slow-%d", i)
		ready := filepath.Join(dir, id)
		script := `trap 'sleep 0.5; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
		if err := m.Add(commandPlan(id, &task.Task{}, "sh", "-c", script)); err != nil {
			t.Fatal(err)
		}
		triggered(t, m, id)
		eventually(t, "the command to get ready", func() bool { _, err := os.Stat(ready); return err == nil })
	}
	begin := time.Now()
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	took := time.Since(begin)
	if finished := sender.finished(); len(finished) != plans {
		t.Fatalf("finished = %+v when Stop returned", finished)
	}
	if took >= plans*500*time.Millisecond {
		t.Fatalf("Stop took %s: it waited for the plans one after another", took)
	}
}

type countingSender struct {
	mu       sync.Mutex
	started  map[string]int
	finished map[string]int
}

func (c *countingSender) Name() string                   { return "counter" }
func (c *countingSender) Dependencies() []common.Service { return nil }

func (c *countingSender) SendMessage(_ context.Context, _ common.Named, msg common.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch ev := msg.(type) {
	case entity.PlanRunStarted:
		c.started[ev.PlanID]++
	case entity.PlanRunFinished:
		c.finished[ev.PlanID]++
	}
}

func (c *countingSender) counts(id string) (started, finished int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started[id], c.finished[id]
}

func TestDeleteUnderStress(t *testing.T) {
	sender := &countingSender{started: map[string]int{}, finished: map[string]int{}}
	m := newManager(sender, startedTaskManager(t, task.MaxConcurrency(4)), WithLogDir(t.TempDir()))
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	tasks := []func() *task.Task{
		func() *task.Task { return &task.Task{} },
		func() *task.Task { return &task.Task{RetryAttempts: 2, RetryDelay: time.Millisecond} },
		func() *task.Task { return &task.Task{RequeueAttempts: 3, RequeueCooldown: time.Millisecond} },
		func() *task.Task { return &task.Task{Schedule: "* * * * * *"} },
		func() *task.Task { return &task.Task{Timeout: time.Millisecond} },
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
				tk := tasks[r.Intn(len(tasks))]()
				tk.Params = entity.CommandParams{Command: commands[r.Intn(len(commands))]}
				if tk.Schedule == "" {
					tk.Manual = true
				}
				plan := &entity.Plan{ID: id, Metadata: labels.Set{"task": id}, Kind: entity.PlanKindCommand, Task: tk}
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
				if _, err := m.Delete(id, r.Intn(2) == 0); err != nil {
					t.Errorf("Delete = %v", err)
					return
				}
				if started, finished := sender.counts(id); started != finished {
					t.Errorf("%d runs started and %d finished when Delete returned", started, finished)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestDeleteWithForceSendsTheKillToAnOwnJob(t *testing.T) {
	m, _ := startedPlanner(t)
	var started, killed atomic.Bool
	plan := ownJobPlan("own", &task.Task{GracePeriod: time.Minute}, func(jc job.Context) error {
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
	if _, err := m.Delete("own", true); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job to see the kill", killed.Load)
}
