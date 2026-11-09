package task

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

// Command is what a command task runs.
type Command struct {
	Argv []string          `json:"argv"` // the executable first
	Dir  string            `json:"dir,omitempty"`
	Env  map[string]string `json:"env,omitempty"` // on top of the process environment
}

// CommandResult is how a command's execution ended.
type CommandResult struct {
	ExitCode int   // -1 if the command never started or died of a signal
	Stopped  error // context.Canceled or context.DeadlineExceeded if it was stopped, nil if it ended on its own
	Killed   bool  // it was sent a SIGKILL: forced, or its grace period ran out
	Err      error // why it could not start, or what waiting on it reported
}

// Failure is nil for a command that ended on its own with status 0.
func (r CommandResult) Failure() error {
	switch {
	case r.Stopped != nil:
		return errors.Wrap(r.Stopped)
	case r.ExitCode == 0:
		return nil
	case r.ExitCode > 0:
		return errors.Newf("exit status %d", r.ExitCode)
	default:
		return errors.Wrap(r.Err)
	}
}

// RunCommand is the body of a command task's job: it runs c in a process group of its own,
// writes its output to out, and returns once it has ended. How the command is stopped:
//
//   - the job's context ends, by a cancel or a timeout: a SIGTERM to the group, then a
//     SIGKILL once the grace period is up;
//   - SignalKill: a SIGKILL at once, during the grace period too;
//   - after a stop, whatever is left in the group goes with the command, which a command
//     that ends on its own does not do to its background children.
//
// Once the command has exited, its output is waited for up to the grace period, so a child
// still holding it open does not hold up the end.
func RunCommand(jc job.Context, c Command, grace time.Duration, out io.Writer) CommandResult {
	ctx := jc.Context()
	if len(c.Argv) == 0 {
		return CommandResult{ExitCode: -1, Err: errors.InvalidArgument.Newf("no command to run")}
	}
	if err := ctx.Err(); err != nil {
		return CommandResult{ExitCode: -1, Stopped: err}
	}
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	p := exec.Command(c.Argv[0], c.Argv[1:]...)
	p.Dir = c.Dir
	p.Env = append(os.Environ(), commandEnv(c.Env)...)
	p.Stdout, p.Stderr = out, out
	p.WaitDelay = grace
	cmdutil.SetProcessGroup(p)
	if err := p.Start(); err != nil {
		return CommandResult{ExitCode: -1, Err: errors.Wrapf(err, "failed to start %s", c.Argv[0])}
	}
	exited := make(chan error, 1)
	go func() { exited <- p.Wait() }()

	var res CommandResult
	stop, kill := ctx.Done(), jc.Signaled(job.SignalKill)
	var graceUp <-chan time.Time
	for {
		select {
		case err := <-exited:
			if res.Stopped != nil {
				_ = cmdutil.KillProcessGroup(p)
			}
			res.ExitCode, res.Err = p.ProcessState.ExitCode(), err
			return res
		case <-stop:
			stop = nil
			res.Stopped = ctx.Err()
			_ = cmdutil.TerminateProcessGroup(p)
			timer := time.NewTimer(grace)
			defer timer.Stop()
			graceUp = timer.C
		case <-graceUp:
			graceUp = nil
			res.Killed = true
			_ = cmdutil.KillProcessGroup(p)
		case <-kill:
			kill = nil
			if res.Stopped == nil {
				res.Stopped = context.Canceled
			}
			res.Killed = true
			_ = cmdutil.KillProcessGroup(p)
		}
	}
}

// NewCommand makes a task whose job runs c, stopped within the task's own GracePeriod. Its
// output goes to out, or nowhere when out is nil. Set the task's schedule and other rules
// before adding it.
func NewCommand(id string, c Command, out io.Writer) *Task {
	t := &Task{}
	t.Job = job.New(id, func(jc job.Context) error {
		return RunCommand(jc, c, t.Grace(), out).Failure()
	})
	return t
}

func commandEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
