package plan

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

func TestCommandPlanRunsOnlyWhenTriggered(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("idle", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events before any trigger: %v", kinds)
	}
}

func TestCommandPlanReportsASuccessfulRun(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("ok", &entity.Task{}, "sh", "-c", "echo hello")); err != nil {
		t.Fatal(err)
	}
	runID := trigger(t, m, "ok")
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
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("bad", &entity.Task{}, "sh", "-c", "echo boom >&2; exit 3")); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "bad")
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
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("missing", &entity.Task{}, "/no/such/binary")); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "missing")
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
	m, sender := startedManager(t)
	plan := commandPlan("slow", &entity.Task{Timeout: 100 * time.Millisecond}, "sleep", "30")
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "slow")
	eventually(t, "the run to time out", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.State != entity.PlanRunFailed {
		t.Fatalf("state = %s", finished.State)
	}
	if finished.Error != "timed out after 100ms" {
		t.Fatalf("error = %q", finished.Error)
	}
}

func TestScheduledCommandPlanReportsItsSource(t *testing.T) {
	m, sender := startedManager(t)
	if err := m.Add(commandPlan("cron", &entity.Task{Schedule: "* * * * * *"}, "true")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a scheduled run", func() bool { return len(sender.finished()) >= 1 })
	finished := sender.finished()[0]
	if finished.Source != entity.PlanRunSchedule || finished.ID == "" {
		t.Fatalf("finished = %+v", finished)
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
			m, sender := startedManager(t)
			if err := m.Add(commandPlan("rerun", tc.task, "false")); err != nil {
				t.Fatal(err)
			}
			runID := trigger(t, m, "rerun")
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

func TestCommandPlanWithoutASenderStillRuns(t *testing.T) {
	m := newManager(nil, startedTaskManager(t), WithLogDir(t.TempDir()))
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	if err := m.Add(commandPlan("silent", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	runID := trigger(t, m, "silent")
	eventually(t, "the run to finish", func() bool {
		run, _ := m.LastRun("silent")
		return run != nil && run.ID == runID && run.State == entity.PlanRunSucceeded
	})
}

func TestCommandPlanKeepsATailAndTheWholeLog(t *testing.T) {
	m, sender := startedManager(t, WithOutputTailBytes(16))
	script := `i=0; while [ $i -lt 20000 ]; do echo line-$i; i=$((i+1)); done`
	if err := m.Add(commandPlan("loud", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "loud")
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
	m, sender := startedManager(t, WithLogDir(""))
	if err := m.Add(commandPlan("nolog", &entity.Task{}, "sh", "-c", "echo hi")); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "nolog")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	finished := sender.finished()[0]
	if finished.LogPath != "" || finished.OutputTail != "hi\n" {
		t.Fatalf("finished = %+v", finished)
	}
}

func TestCommandPlanAppliesDirAndEnv(t *testing.T) {
	m, sender := startedManager(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLAN_TEST_INHERITED", "kept")
	t.Setenv("PLAN_TEST_OVERRIDDEN", "old")
	plan := commandPlan("env", &entity.Task{}, "sh", "-c", `pwd; echo "$PLAN_TEST_INHERITED $PLAN_TEST_OVERRIDDEN"`)
	plan.Params = jobs.Command{
		Argv: plan.Params.(jobs.Command).Argv,
		Dir:  dir,
		Env:  map[string]string{"PLAN_TEST_OVERRIDDEN": "new"},
	}
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "env")
	eventually(t, "the run to finish", func() bool { return len(sender.finished()) == 1 })
	want := dir + "\nkept new\n"
	if got := sender.finished()[0].OutputTail; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCommandPlanCancelKillsTheWholeProcessGroup(t *testing.T) {
	m, sender := startedManager(t)
	marker := filepath.Join(t.TempDir(), "survived")
	script := "(sleep 1; touch " + marker + ") & echo started; wait"
	if err := m.Add(commandPlan("group", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "group")
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

func TestNoProcessOutlivesAStoppedRun(t *testing.T) {
	const grace = 200 * time.Millisecond
	stops := []struct {
		name  string
		task  func() *entity.Task
		stop  func(*testing.T, *manager)
		state entity.PlanRunState
	}{
		{
			name: "Cancel", task: func() *entity.Task { return &entity.Task{} }, state: entity.PlanRunCanceled,
			stop: func(t *testing.T, m *manager) {
				if err := m.Cancel("stubborn"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Delete", task: func() *entity.Task { return &entity.Task{} }, state: entity.PlanRunCanceled,
			stop: func(t *testing.T, m *manager) {
				if _, err := m.Delete(context.Background(), "stubborn"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a timeout", task: func() *entity.Task { return &entity.Task{Timeout: 300 * time.Millisecond} }, state: entity.PlanRunFailed,
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
				m, sender := startedManager(t)
				pids := pidFile(t)
				if err := m.Add(withGrace(commandPlan("stubborn", tc.task(), "sh", "-c", sc.script, pids), grace)); err != nil {
					t.Fatal(err)
				}
				trigger(t, m, "stubborn")
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
	m, sender := startedManager(t)
	pids := pidFile(t)
	plan := withGrace(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids), 100*time.Millisecond)
	if err := m.Add(plan); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "stubborn")
	waitPid(t, pids)
	begin := time.Now()
	if err := m.Cancel("stubborn"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the run to end", func() bool { return len(sender.finished()) == 1 })
	assertWithin(t, "the canceled run", 2*time.Second, begin)
}

func TestZeroGracePeriodStillGivesACommandTimeToCleanUp(t *testing.T) {
	m, sender := startedManager(t)
	ready := filepath.Join(t.TempDir(), "ready")
	script := `trap 'sleep 0.3; echo cleaned; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	if err := m.Add(commandPlan("tidy", &entity.Task{}, "sh", "-c", script)); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "tidy")
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
	m, sender := startedManager(t)
	pids := pidFile(t)
	script := `sleep 60 >/dev/null 2>&1 & echo $! > "$0"`
	if err := m.Add(commandPlan("daemonizer", &entity.Task{}, "sh", "-c", script, pids)); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "daemonizer")
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
