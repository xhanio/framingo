package task

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/job"
)

const (
	// each writes its pid to "$0" once it ignores SIGTERM
	stubbornChild   = `sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" & wait`
	stubbornCommand = `trap "" TERM; ` + stubbornChild
)

type safeBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func commandRun(ctx context.Context, c Command, grace time.Duration) (job.Job, *safeBuffer, <-chan CommandResult) {
	out := &safeBuffer{}
	ended := make(chan CommandResult, 1)
	c.Grace, c.Out = grace, out
	j := job.New("command", func(jc job.Context) error {
		ended <- c.execute(jc)
		return nil
	})
	j.Run(ctx, nil)
	return j, out, ended
}

func sh(script string, args ...string) Command {
	return Command{Argv: append([]string{"sh", "-c", script}, args...)}
}

func pidPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "pid")
}

func readPid(t *testing.T, path string) int {
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
func processGone(pid int) bool {
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
	eventually(t, "process "+strconv.Itoa(pid)+" to be gone", func() bool { return processGone(pid) })
}

func result(t *testing.T, ended <-chan CommandResult) CommandResult {
	t.Helper()
	select {
	case r := <-ended:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("the command did not end")
		return CommandResult{}
	}
}

func TestCommandEndingOnItsOwn(t *testing.T) {
	cases := []struct {
		name      string
		script    string
		code      int
		succeeded bool
	}{
		{name: "successfully", script: "echo hi", code: 0, succeeded: true},
		{name: "with a failing status", script: "echo hi; exit 3", code: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, out, ended := commandRun(context.Background(), sh(tc.script), time.Second)
			r := result(t, ended)
			if r.ExitCode != tc.code || r.Stopped != nil || r.Killed {
				t.Fatalf("result = %+v", r)
			}
			if out.String() != "hi\n" {
				t.Fatalf("output = %q", out.String())
			}
			if r.succeeded() != tc.succeeded {
				t.Fatalf("succeeded = %v, want %v", r.succeeded(), tc.succeeded)
			}
		})
	}
}

func TestCommandTakesItsDirAndEnv(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_TEST_INHERITED", "kept")
	t.Setenv("TASK_TEST_OVERRIDDEN", "old")
	c := sh(`pwd; echo "$TASK_TEST_INHERITED $TASK_TEST_OVERRIDDEN"`)
	c.Dir, c.Env = dir, map[string]string{"TASK_TEST_OVERRIDDEN": "new"}
	_, out, ended := commandRun(context.Background(), c, time.Second)
	result(t, ended)
	if want := dir + "\nkept new\n"; out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestCommandThatCannotStart(t *testing.T) {
	_, _, ended := commandRun(context.Background(), Command{Argv: []string{"/no/such/binary"}}, time.Second)
	r := result(t, ended)
	if r.ExitCode != -1 || r.Err == nil || r.Stopped != nil || r.succeeded() {
		t.Fatalf("result = %+v", r)
	}
}

func TestCommandWithoutArgvIsRejected(t *testing.T) {
	_, _, ended := commandRun(context.Background(), Command{}, time.Second)
	if r := result(t, ended); !errors.Is(r.Err, errors.InvalidArgument) {
		t.Fatalf("result = %+v", r)
	}
}

func TestCommandIsNotStartedOnceItsContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	marker := filepath.Join(t.TempDir(), "ran")
	_, _, ended := commandRun(ctx, sh("touch "+marker), time.Second)
	r := result(t, ended)
	if !errors.Is(r.Stopped, context.Canceled) || r.ExitCode != -1 {
		t.Fatalf("result = %+v", r)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the command ran")
	}
}

func TestCanceledCommandGetsItsGracePeriodToCleanUp(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	script := `trap 'sleep 0.3; echo cleaned; exit 0' TERM; sleep 30 & touch ` + ready + `; wait`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, out, ended := commandRun(ctx, sh(script), 5*time.Second)
	eventually(t, "the command to get ready", func() bool { _, err := os.Stat(ready); return err == nil })
	cancel()
	r := result(t, ended)
	if !errors.Is(r.Stopped, context.Canceled) || r.Killed {
		t.Fatalf("result = %+v", r)
	}
	if out.String() != "cleaned\n" {
		t.Fatalf("output = %q: the command was cut off while cleaning up", out.String())
	}
}

func TestGracePeriodRunsOutOnACommandIgnoringTheTerm(t *testing.T) {
	pids := pidPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _, ended := commandRun(ctx, sh(stubbornCommand, pids), 200*time.Millisecond)
	child := readPid(t, pids)
	begin := time.Now()
	cancel()
	r := result(t, ended)
	if took := time.Since(begin); took < 200*time.Millisecond || took > 2*time.Second {
		t.Fatalf("the command ended %s after the cancel, want about the 200ms grace period", took)
	}
	if !errors.Is(r.Stopped, context.Canceled) || !r.Killed {
		t.Fatalf("result = %+v", r)
	}
	assertGone(t, child)
}

func TestNoProcessOutlivesAStoppedCommand(t *testing.T) {
	scripts := []struct {
		name   string
		script string
	}{
		{name: "a stubborn child", script: stubbornChild},
		{name: "a stubborn child without the output", script: `sh -c 'trap "" TERM; echo $$ > "$0"; exec sleep 60' "$0" >/dev/null 2>&1 & wait`},
		{name: "a stubborn command", script: stubbornCommand},
	}
	stops := []struct {
		name    string
		timeout time.Duration
		stop    func(context.CancelFunc, job.Job)
		want    error
	}{
		{name: "canceled", stop: func(cancel context.CancelFunc, _ job.Job) { cancel() }, want: context.Canceled},
		{name: "timed out", timeout: 300 * time.Millisecond, stop: func(context.CancelFunc, job.Job) {}, want: context.DeadlineExceeded},
		{name: "killed", stop: func(_ context.CancelFunc, j job.Job) { j.Signal(job.SignalKill) }, want: context.Canceled},
	}
	for _, sc := range scripts {
		for _, st := range stops {
			t.Run(sc.name+"/"+st.name, func(t *testing.T) {
				pids := pidPath(t)
				ctx, cancel := context.WithCancel(context.Background())
				if st.timeout > 0 {
					ctx, cancel = context.WithTimeout(context.Background(), st.timeout)
				}
				defer cancel()
				j, _, ended := commandRun(ctx, sh(sc.script, pids), 200*time.Millisecond)
				child := readPid(t, pids)
				st.stop(cancel, j)
				r := result(t, ended)
				if !errors.Is(r.Stopped, st.want) {
					t.Fatalf("stopped by %v, want %v", r.Stopped, st.want)
				}
				assertGone(t, child)
			})
		}
	}
}

func TestKillSignalCutsTheGracePeriodShort(t *testing.T) {
	pids := pidPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j, _, ended := commandRun(ctx, sh(stubbornCommand, pids), time.Minute)
	child := readPid(t, pids)
	cancel()
	time.Sleep(100 * time.Millisecond)
	begin := time.Now()
	if !j.Signal(job.SignalKill) {
		t.Fatal("Signal = false while the command runs")
	}
	r := result(t, ended)
	if took := time.Since(begin); took > time.Second {
		t.Fatalf("the command ended %s after the kill", took)
	}
	if !errors.Is(r.Stopped, context.Canceled) || !r.Killed {
		t.Fatalf("result = %+v", r)
	}
	assertGone(t, child)
}

func TestCommandEndingOnItsOwnKeepsItsBackgroundChild(t *testing.T) {
	pids := pidPath(t)
	_, _, ended := commandRun(context.Background(), sh(`sleep 60 >/dev/null 2>&1 & echo $! > "$0"`, pids), time.Second)
	r := result(t, ended)
	if !r.succeeded() {
		t.Fatalf("result = %+v", r)
	}
	child := readPid(t, pids)
	time.Sleep(50 * time.Millisecond)
	if processGone(child) {
		t.Fatal("a command that ended on its own took its background child with it")
	}
}

func TestChildHoldingTheOutputDoesNotFailACommandThatEndedOnItsOwn(t *testing.T) {
	pids := pidPath(t)
	begin := time.Now()
	_, _, ended := commandRun(context.Background(), sh(`sleep 60 & echo $! > "$0"`, pids), 200*time.Millisecond)
	r := result(t, ended)
	readPid(t, pids)
	if took := time.Since(begin); took > 3*time.Second {
		t.Fatalf("the command took %s to end", took)
	}
	if r.ExitCode != 0 || !r.succeeded() {
		t.Fatalf("result = %+v", r)
	}
}

func TestCommandRunIsATaskJobThatHonorsItsGracePeriod(t *testing.T) {
	s := startedScheduler(t)
	pids := pidPath(t)
	c := sh(stubbornCommand, pids)
	c.Grace = 200 * time.Millisecond
	tk := &Task{Job: job.New("stubborn", c.Run), Manual: true}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	if err := s.Trigger(tk.ID(), nil); err != nil {
		t.Fatal(err)
	}
	child := readPid(t, pids)
	begin := time.Now()
	assertClosed(t, s.Remove(tk))
	if took := time.Since(begin); took < 200*time.Millisecond || took > 2*time.Second {
		t.Fatalf("Remove's end came %s after it, want about the 200ms grace period", took)
	}
	assertGone(t, child)
}

func TestCommandRunFailsTheJobAsTheCommandFails(t *testing.T) {
	s := startedScheduler(t)
	tk := &Task{Job: job.New("failing", sh("exit 4").Run)}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job to fail", func() bool { return tk.Job.IsState(job.StateFailed) })
	err := tk.Job.Err()
	var failed *CommandError
	if !errors.As(err, &failed) || failed.Result.ExitCode != 4 || err.Error() != "exit status 4" {
		t.Fatalf("job error = %v", err)
	}
}

func runOnce(ctx context.Context, c Command) error {
	j := job.New("once", c.Run)
	j.Run(ctx, nil)
	j.Wait()
	return j.Err()
}

func TestRunErrorCarriesTheResult(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	timedOut, cancelTimeout := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelTimeout()
	cases := []struct {
		name    string
		ctx     context.Context
		command Command
		message string
		is      error
		check   func(CommandResult) bool
	}{
		{
			name: "a failing status", ctx: context.Background(), command: sh("exit 3"), message: "exit status 3",
			check: func(r CommandResult) bool { return r.ExitCode == 3 && r.Stopped == nil },
		},
		{
			name: "a cancel", ctx: canceled, command: sh("true"), message: "context canceled", is: context.Canceled,
			check: func(r CommandResult) bool { return errors.Is(r.Stopped, context.Canceled) && r.ExitCode == -1 },
		},
		{
			name: "a timeout", ctx: timedOut, command: sh("sleep 30"), message: "context deadline exceeded", is: context.DeadlineExceeded,
			check: func(r CommandResult) bool { return errors.Is(r.Stopped, context.DeadlineExceeded) },
		},
		{
			name: "a command that cannot start", ctx: context.Background(), command: Command{Argv: []string{"/no/such/binary"}}, message: "/no/such/binary",
			check: func(r CommandResult) bool { return r.ExitCode == -1 && r.Err != nil },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runOnce(tc.ctx, tc.command)
			var failed *CommandError
			if !errors.As(err, &failed) {
				t.Fatalf("error = %v (%T), want a *CommandError", err, err)
			}
			if !tc.check(failed.Result) {
				t.Fatalf("result = %+v", failed.Result)
			}
			if !strings.Contains(err.Error(), tc.message) || strings.Contains(err.Error(), "\n") {
				t.Fatalf("message = %q, want one line with %q", err.Error(), tc.message)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tc.is)
			}
		})
	}
}

func TestRunErrorSurvivesWrapping(t *testing.T) {
	err := errors.Wrapf(errors.Wrap(runOnce(context.Background(), sh("exit 5"))), "plan %s run %s", "p", "r")
	var failed *CommandError
	if !errors.As(err, &failed) || failed.Result.ExitCode != 5 {
		t.Fatalf("errors.As through the wrapping = %v, %+v", failed != nil, failed)
	}
}

func TestRunOfASuccessIsNil(t *testing.T) {
	if err := runOnce(context.Background(), sh("true")); err != nil {
		t.Fatalf("Run = %v", err)
	}
}

func TestCommandRunWritesToItsOut(t *testing.T) {
	out := &safeBuffer{}
	c := sh("echo hi")
	c.Out = out
	j := job.New("out", c.Run)
	j.Run(context.Background(), nil)
	j.Wait()
	if err := j.Err(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hi\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestCommandIsFixedOnceItsRunIsTaken(t *testing.T) {
	first, later := &safeBuffer{}, &safeBuffer{}
	c := sh("echo hi")
	c.Out = first
	j := job.New("fixed", c.Run)
	c.Argv, c.Out = []string{"false"}, later
	j.Run(context.Background(), nil)
	j.Wait()
	if err := j.Err(); err != nil || first.String() != "hi\n" || later.String() != "" {
		t.Fatalf("err %v, first %q, later %q: a change after the job was made reached it", err, first.String(), later.String())
	}
}
