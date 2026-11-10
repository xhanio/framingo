package task

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

// Command is a command task. job.New(id, cmd.Run) takes a copy, which fixes the definition.
type Command struct {
	Argv  []string          `json:"argv"` // the executable first
	Dir   string            `json:"dir,omitempty"`
	Env   map[string]string `json:"env,omitempty"`   // on top of the process environment
	Grace time.Duration     `json:"grace,omitempty"` // how long a stopped command gets before a SIGKILL; zero is DefaultGracePeriod
	Out   io.Writer         `json:"-"`               // nil discards the output
}

type CommandResult struct {
	ExitCode int   // -1 if the command never started or died of a signal
	Stopped  error // context.Canceled or context.DeadlineExceeded if it was stopped, nil if it ended on its own
	Killed   bool  // it was sent a SIGKILL: forced, or its grace period ran out
	Err      error // why it could not start, or what waiting on it reported
}

func (r CommandResult) succeeded() bool {
	return r.Stopped == nil && r.ExitCode == 0
}

// CommandError unwraps to what stopped the command, so errors.Is(err, context.Canceled) works.
type CommandError struct {
	Result CommandResult
}

func (e *CommandError) Error() string {
	if err := e.Unwrap(); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Result.ExitCode)
}

func (e *CommandError) Unwrap() error {
	if e.Result.Stopped != nil {
		return e.Result.Stopped
	}
	return e.Result.Err
}

// Run returns nil when the command ended on its own with status 0, a *CommandError otherwise.
// A job function with bookkeeping of its own calls it on a copy with the Out it wants.
func (c Command) Run(jc job.Context) error {
	if res := c.execute(jc); !res.succeeded() {
		return &CommandError{Result: res}
	}
	return nil
}

func (c Command) execute(jc job.Context) CommandResult {
	ctx := jc.Context()
	if len(c.Argv) == 0 {
		return CommandResult{ExitCode: -1, Err: errors.InvalidArgument.Newf("no command to run")}
	}
	if err := ctx.Err(); err != nil {
		return CommandResult{ExitCode: -1, Stopped: err}
	}
	grace := c.Grace
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	p := exec.Command(c.Argv[0], c.Argv[1:]...)
	p.Dir = c.Dir
	p.Env = append(os.Environ(), commandEnv(c.Env)...)
	p.Stdout, p.Stderr = c.Out, c.Out
	p.WaitDelay = grace // so a child still holding the output does not hold up the end
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
				// what ignored the SIGTERM goes too; a command that ended on its own keeps its children
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

func commandEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
