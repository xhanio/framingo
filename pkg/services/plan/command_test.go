package plan

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/services/task"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

type recordingSender struct {
	mu   sync.Mutex
	msgs []common.Message
}

func (r *recordingSender) Name() string                   { return "recorder" }
func (r *recordingSender) Dependencies() []common.Service { return nil }

func (r *recordingSender) SendMessage(_ context.Context, _ common.Named, msg common.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msg)
}

func (r *recordingSender) started() []entity.PlanRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.PlanRun
	for _, msg := range r.msgs {
		if ev, ok := msg.(entity.PlanRunStarted); ok {
			out = append(out, ev.PlanRun)
		}
	}
	return out
}

func (r *recordingSender) finished() []entity.PlanRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.PlanRun
	for _, msg := range r.msgs {
		if ev, ok := msg.(entity.PlanRunFinished); ok {
			out = append(out, ev.PlanRun)
		}
	}
	return out
}

func (r *recordingSender) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, msg := range r.msgs {
		out = append(out, msg.Kind())
	}
	return out
}

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

func startedTaskManager(t *testing.T, opts ...task.Option) task.Manager {
	t.Helper()
	tm := task.New(opts...)
	if err := tm.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tm.Stop(context.Background()) })
	return tm
}

func startedPlanner(t *testing.T, opts ...Option) (*manager, *recordingSender) {
	t.Helper()
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t), append([]Option{WithLogDir(t.TempDir())}, opts...)...)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	return m, sender
}

func commandPlan(id string, tk *entity.Task, command ...string) *entity.Plan {
	if tk.Schedule == "" {
		tk.Manual = true
	}
	return &entity.Plan{ID: id, Metadata: labels.Set{"task": id}, Kind: entity.PlanKindCommand, Params: jobs.Command{Argv: command}, Task: tk}
}

func withGrace(plan *entity.Plan, grace time.Duration) *entity.Plan {
	cmd := plan.Params.(jobs.Command)
	cmd.Grace = grace
	plan.Params = cmd
	return plan
}

func triggered(t *testing.T, m *manager, id string) string {
	t.Helper()
	runID, err := m.Trigger(id, entity.PlanRunManual)
	if err != nil {
		t.Fatalf("Trigger = %v", err)
	}
	if runID == "" {
		t.Fatal("Trigger of a command plan returned no run ID")
	}
	return runID
}

func TestCommandPlanRunsOnlyWhenTriggered(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("idle", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events before any trigger: %v", kinds)
	}
}

func TestCommandPlanReportsASuccessfulRun(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("ok", &entity.Task{}, "sh", "-c", "echo hello")); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "ok")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })

	kinds := sender.kinds()
	if len(kinds) != 2 || kinds[0] != "plan.run.started" || kinds[1] != "plan.run.finished" {
		t.Fatalf("event kinds = %v", kinds)
	}
	started, finished := sender.started()[0], sender.finished()[0]
	if started.ID != runID || finished.ID != runID {
		t.Fatalf("run IDs = %q and %q, want %q from Trigger", started.ID, finished.ID, runID)
	}
	if started.State != entity.PlanRunRunning {
		t.Fatalf("started state = %s", started.State)
	}
	if started.PlanID != "ok" || started.Source != entity.PlanRunManual || started.StartedAt.IsZero() {
		t.Fatalf("started = %+v", started)
	}
	if finished.State != entity.PlanRunSucceeded || finished.ExitCode != 0 || finished.Error != "" {
		t.Fatalf("finished = %+v", finished)
	}
	if finished.StartedAt.IsZero() || finished.EndedAt.Before(finished.StartedAt) {
		t.Fatalf("times = %s to %s", finished.StartedAt, finished.EndedAt)
	}
	if finished.OutputTail != "hello\n" {
		t.Fatalf("output tail = %q", finished.OutputTail)
	}
	raw, err := os.ReadFile(finished.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "hello\n") {
		t.Fatalf("log file = %q", raw)
	}
	if started.LogPath != finished.LogPath {
		t.Fatalf("log paths = %q and %q", started.LogPath, finished.LogPath)
	}
	if !strings.HasPrefix(finished.LogPath, m.logDir+string(filepath.Separator)+"ok"+string(filepath.Separator)) {
		t.Fatalf("log path = %q, want it under the plan's directory in %q", finished.LogPath, m.logDir)
	}
}

func TestCommandPlanReportsAFailedRun(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("bad", &entity.Task{}, "sh", "-c", "echo boom >&2; exit 3")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "bad")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunFailed || finished.ExitCode != 3 {
		t.Fatalf("finished = %+v", finished)
	}
	if finished.Error != "exit status 3" {
		t.Fatalf("error = %q", finished.Error)
	}
	if finished.OutputTail != "boom\n" {
		t.Fatalf("output tail = %q", finished.OutputTail)
	}
}

func TestCommandPlanReportsACommandThatCannotStart(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("missing", &entity.Task{}, "/no/such/binary")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "missing")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunFailed || finished.ExitCode != -1 {
		t.Fatalf("finished = %+v", finished)
	}
	if !strings.Contains(finished.Error, "/no/such/binary") {
		t.Fatalf("error = %q", finished.Error)
	}
}

func TestCommandPlanReportsATimeout(t *testing.T) {
	m, sender := startedPlanner(t)
	plan := commandPlan("slow", &entity.Task{Timeout: 100 * time.Millisecond}, "sleep", "30")
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "slow")
	eventually(t, "the run to time out", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunFailed {
		t.Fatalf("state = %s", finished.State)
	}
	if finished.Error != "timed out after 100ms" {
		t.Fatalf("error = %q", finished.Error)
	}
}

func TestCommandPlanCancelKillsTheWholeProcessGroup(t *testing.T) {
	m, sender := startedPlanner(t)
	marker := filepath.Join(t.TempDir(), "survived")
	script := "(sleep 1; touch " + marker + ") & echo started; wait"
	if err := m.Add(commandPlan("group", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "group")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	time.Sleep(100 * time.Millisecond)
	if err := m.Cancel("group"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunCanceled || finished.Error != "canceled" {
		t.Fatalf("finished = %+v", finished)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a child of the canceled command kept running")
	}
}

func TestDeleteOfARunningPlanEndsItCanceled(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("deleted", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "deleted")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	time.Sleep(50 * time.Millisecond)
	if _, err := m.Delete(canceled(), "deleted"); !errors.Is(err, context.Canceled) {
		t.Fatalf("forced Delete = %v, want the context's error for the run it left", err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	if st := sender.finished()[0].State; st != entity.PlanRunCanceled {
		t.Fatalf("state = %s", st)
	}
}

func TestCommandPlanAppliesDirAndEnv(t *testing.T) {
	m, sender := startedPlanner(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLANNER_TEST_INHERITED", "kept")
	t.Setenv("PLANNER_TEST_OVERRIDDEN", "old")
	plan := commandPlan("env", &entity.Task{}, "sh", "-c", `pwd; echo "$PLANNER_TEST_INHERITED $PLANNER_TEST_OVERRIDDEN"`)
	plan.Params = jobs.Command{
		Argv: plan.Params.(jobs.Command).Argv,
		Dir:  dir,
		Env:  map[string]string{"PLANNER_TEST_OVERRIDDEN": "new"},
	}
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "env")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	want := dir + "\nkept new\n"
	if got := sender.finished()[0].OutputTail; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCommandPlanKeepsATailAndTheWholeLog(t *testing.T) {
	m, sender := startedPlanner(t, WithOutputTailBytes(16))
	script := `i=0; while [ $i -lt 20000 ]; do echo line-$i; i=$((i+1)); done`
	if err := m.Add(commandPlan("loud", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "loud")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunSucceeded {
		t.Fatalf("finished = %+v", finished)
	}
	if len(finished.OutputTail) != 16 || !strings.HasSuffix(finished.OutputTail, "line-19999\n") {
		t.Fatalf("output tail = %q", finished.OutputTail)
	}
	raw, err := os.ReadFile(finished.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "line-0\n") || !strings.HasSuffix(string(raw), "line-19999\n") {
		t.Fatalf("log file of %d bytes lost output", len(raw))
	}
}

func TestCommandPlanWithoutALogDirKeepsNoFile(t *testing.T) {
	m, sender := startedPlanner(t, WithLogDir(""))
	if err := m.Add(commandPlan("nolog", &entity.Task{}, "sh", "-c", "echo hi")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "nolog")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.LogPath != "" || finished.OutputTail != "hi\n" {
		t.Fatalf("finished = %+v", finished)
	}
}

func TestScheduledCommandPlanReportsItsSource(t *testing.T) {
	m, sender := startedPlanner(t)
	if err := m.Add(commandPlan("cron", &entity.Task{Schedule: "* * * * * *"}, "true")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a scheduled run", func() bool { return len(sender.finished()) >= 1 })
	finished := sender.finished()[0]
	if finished.Source != entity.PlanRunSchedule || finished.ID == "" {
		t.Fatalf("finished = %+v", finished)
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
			m, sender := startedPlanner(t)
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
	m, sender := startedPlanner(t)
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
	m, sender := startedPlanner(t)
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

func TestEveryRerunIsANewRun(t *testing.T) {
	cases := []struct {
		name string
		task *entity.Task
	}{
		{name: "a retry", task: &entity.Task{RetryAttempts: 1, RetryDelay: time.Millisecond}},
		{name: "a requeue", task: &entity.Task{RequeueAttempts: 1, RequeueCooldown: 10 * time.Millisecond}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, sender := startedPlanner(t)
			if err := m.Add(commandPlan("rerun", tc.task, "false")); err != nil {
				t.Fatal(err)
			}
			runID := triggered(t, m, "rerun")
			eventually(t, "both runs to finish", func() bool { return len(sender.finished()) == 2 })
			runs := sender.finished()
			if runs[0].ID != runID {
				t.Fatalf("first run = %q, want %q from Trigger", runs[0].ID, runID)
			}
			if runs[1].ID == "" || runs[1].ID == runID {
				t.Fatalf("second run ID = %q, want a new one", runs[1].ID)
			}
			if runs[0].Source != entity.PlanRunManual || runs[1].Source != entity.PlanRunManual {
				t.Fatalf("sources = %s, %s", runs[0].Source, runs[1].Source)
			}
		})
	}
}

func TestResultFollowsTheLatestRun(t *testing.T) {
	m, sender := startedPlanner(t)
	release := filepath.Join(t.TempDir(), "release")
	script := "while [ ! -e " + release + " ]; do sleep 0.01; done"
	if err := m.Add(commandPlan("latest", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	if res, err := m.Result("latest"); err != nil || res != nil {
		t.Fatalf("Result before any run = %v, %v", res, err)
	}
	runID := triggered(t, m, "latest")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	res, err := m.Result("latest")
	if err != nil {
		t.Fatal(err)
	}
	run, ok := res.(entity.PlanRun)
	if !ok || run.ID != runID || run.State != entity.PlanRunRunning {
		t.Fatalf("Result during the run = %+v", res)
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	res, err = m.Result("latest")
	if err != nil {
		t.Fatal(err)
	}
	run, ok = res.(entity.PlanRun)
	if !ok || run.ID != runID || run.State != entity.PlanRunSucceeded {
		t.Fatalf("Result after the run = %+v", res)
	}
}

func TestLastRunOfAnUnknownPlanIsNotFound(t *testing.T) {
	m, _ := startedPlanner(t)
	if _, err := m.LastRun("nobody"); !errors.Is(err, errors.NotFound) {
		t.Fatalf("LastRun = %v, want NotFound", err)
	}
}

func TestLastRunFollowsTheLatestRun(t *testing.T) {
	m, sender := startedPlanner(t)
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
	m, _ := startedPlanner(t)
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
			m, _ := startedPlanner(t)
			if err := m.Add(tc.plan); !errors.Is(err, errors.InvalidArgument) {
				t.Fatalf("Add = %v, want InvalidArgument", err)
			}
			if _, err := m.Trigger("x", entity.PlanRunManual); !errors.Is(err, errors.NotFound) {
				t.Fatalf("a rejected plan stayed: Trigger = %v", err)
			}
		})
	}
}

func TestTriggerAndCancelErrors(t *testing.T) {
	m, sender := startedPlanner(t)
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

func TestPlanWithItsOwnJobStillWorks(t *testing.T) {
	m, sender := startedPlanner(t)
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

func TestCancelOfAFinishedOneShotPlanConflicts(t *testing.T) {
	m, _ := startedPlanner(t)
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

func TestPlannerDependsOnItsManagerAndSender(t *testing.T) {
	sender := &recordingSender{}
	tm := task.New()
	deps := newManager(sender, tm).Dependencies()
	if !slices.Contains(deps, common.Service(tm)) || !slices.Contains(deps, common.Service(sender)) {
		t.Fatalf("dependencies = %v", deps)
	}
	if deps := newManager(nil, tm).Dependencies(); len(deps) != 1 || deps[0] != common.Service(tm) {
		t.Fatalf("dependencies without a sender = %v", deps)
	}
}

func TestInjectedManagerSetsTheConcurrency(t *testing.T) {
	sender := &recordingSender{}
	m := newManager(sender, startedTaskManager(t, task.MaxConcurrency(1)), WithLogDir(t.TempDir()))
	if err := m.Add(commandPlan("first", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("second", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "first")
	eventually(t, "the first run to start", func() bool { return len(sender.started()) == 1 })
	triggered(t, m, "second")
	time.Sleep(100 * time.Millisecond)
	if started := sender.started(); len(started) != 1 {
		t.Fatalf("started = %+v, want the second plan held back by the limit", started)
	}
	if err := m.Cancel("first"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second run", func() bool { return len(sender.finished()) == 2 })
}

func TestPlannerStartDoesNotStartTheManager(t *testing.T) {
	sender := &recordingSender{}
	m := newManager(sender, task.New(), WithLogDir(t.TempDir()))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	if err := m.Add(commandPlan("idle", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "idle")
	time.Sleep(50 * time.Millisecond)
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events from a manager nobody started: %v", kinds)
	}
}

func TestPlannerStopEndsItsPlansAndLeavesTheManagerRunning(t *testing.T) {
	sender := &recordingSender{}
	tm := startedTaskManager(t)
	release := make(chan struct{})
	defer close(release)
	other := &entity.Task{Job: job.New("other", func(job.Context) error { <-release; return nil }), Manual: true}
	if err := tm.Add(other); err != nil {
		t.Fatal(err)
	}
	if err := tm.Trigger(other.ID(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the other task to start", other.Job.IsExecuting)

	m := newManager(sender, tm, WithLogDir(t.TempDir()))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("mine", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "mine")
	eventually(t, "the run to start", func() bool { return len(sender.started()) == 1 })
	time.Sleep(50 * time.Millisecond)
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	if st := sender.finished()[0].State; st != entity.PlanRunCanceled {
		t.Fatalf("state = %s", st)
	}
	if _, err := m.Trigger("mine", entity.PlanRunManual); !errors.Is(err, errors.NotFound) {
		t.Fatalf("Trigger after Stop = %v, want NotFound", err)
	}
	if !other.Job.IsExecuting() {
		t.Fatalf("the other task's state = %s after the plan manager stopped", other.Job.State())
	}
	var runs atomic.Int32
	if err := tm.Add(&entity.Task{Job: job.New("later", func(job.Context) error { runs.Add(1); return nil })}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the manager to run a new task", func() bool { return runs.Load() == 1 })
}

func TestPlanIDTakenOnTheManagerIsAConflict(t *testing.T) {
	m, _ := startedPlanner(t)
	taken := &entity.Task{Job: job.New("taken", func(job.Context) error { return nil }), Manual: true}
	if err := m.tm.Add(taken); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("taken", &entity.Task{}, "true")); !errors.Is(err, errors.Conflict) {
		t.Fatalf("Add = %v, want Conflict", err)
	}
}

func TestCommandPlanWithoutASenderStillRuns(t *testing.T) {
	m := newManager(nil, startedTaskManager(t), WithLogDir(t.TempDir()))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	if err := m.Add(commandPlan("silent", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	runID := triggered(t, m, "silent")
	eventually(t, "the run to finish", func() bool {
		res, _ := m.Result("silent")
		run, ok := res.(entity.PlanRun)
		return ok && run.ID == runID && run.State == entity.PlanRunSucceeded
	})
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
	m, sender := startedPlanner(t)
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
