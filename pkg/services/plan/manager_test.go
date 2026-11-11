package plan

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

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

func (r *recordingSender) counts(id string) (started, finished int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, msg := range r.msgs {
		switch ev := msg.(type) {
		case entity.PlanRunStarted:
			if ev.PlanID == id {
				started++
			}
		case entity.PlanRunFinished:
			if ev.PlanID == id {
				finished++
			}
		}
	}
	return started, finished
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

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
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

// startedManager starts a plan manager on a task manager of its own.
func startedManager(t *testing.T, opts ...Option) (*manager, *recordingSender) {
	t.Helper()
	return startedManagerOn(t, startedTaskManager(t), opts...)
}

func startedManagerOn(t *testing.T, tm task.Manager, opts ...Option) (*manager, *recordingSender) {
	t.Helper()
	sender := &recordingSender{}
	m := newManager(sender, tm, append([]Option{WithLogDir(t.TempDir())}, opts...)...)
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

func ownJobPlan(id string, tk *entity.Task, fn job.Func) *entity.Plan {
	tk.Job = job.New(id+"-job", fn)
	tk.Manual = true
	return &entity.Plan{ID: id, Task: tk}
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

func TestDependenciesAreTheTaskManagerAndTheSender(t *testing.T) {
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

func TestTheTaskManagerSetsTheConcurrency(t *testing.T) {
	m, sender := startedManagerOn(t, startedTaskManager(t, task.MaxConcurrency(1)))
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
