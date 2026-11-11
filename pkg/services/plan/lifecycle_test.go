package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/services/task"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestStartDoesNotStartTheTaskManager(t *testing.T) {
	m, sender := startedManagerOn(t, task.New())
	if err := m.Add(commandPlan("idle", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	triggered(t, m, "idle")
	time.Sleep(50 * time.Millisecond)
	if kinds := sender.kinds(); len(kinds) != 0 {
		t.Fatalf("events from a task manager nobody started: %v", kinds)
	}
}

func TestStopEndsItsPlansAndLeavesTheTaskManagerRunning(t *testing.T) {
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

	m, sender := startedManagerOn(t, tm)
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
	eventually(t, "the task manager to run a new task", func() bool { return runs.Load() == 1 })
}

func TestStopWaitsForTheSlowestPlanNotTheSum(t *testing.T) {
	m, sender := startedManager(t)
	dir := t.TempDir()
	const plans = 3
	for i := range plans {
		id := fmt.Sprintf("slow-%d", i)
		ready := filepath.Join(dir, id)
		script := `trap 'sleep 0.5; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
		if err := m.Add(commandPlan(id, &entity.Task{}, "sh", "-c", script)); err != nil {
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

func TestStopKillsWhatStillRunsOnceItsContextEnds(t *testing.T) {
	m, _ := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(withGrace(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids), time.Minute)); err != nil {
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

func TestAForcedStopKills(t *testing.T) {
	m, _ := startedManager(t)
	pids := pidFile(t)
	if err := m.Add(commandPlan("stubborn", &entity.Task{}, "sh", "-c", stubbornCommand, pids)); err != nil {
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
