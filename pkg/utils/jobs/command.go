package jobs

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

// DefaultCommandGrace is how long a stopped Command gets before a SIGKILL when its Grace is zero.
const DefaultCommandGrace = 10 * time.Second

// Command is a predefined job function that runs a command: job.New(id, cmd.Run) takes a copy,
// which fixes the definition.
type Command struct {
	Argv  []string          `json:"argv"` // the executable first
	Dir   string            `json:"dir,omitempty"`
	Env   map[string]string `json:"env,omitempty"`   // on top of the process environment
	Grace time.Duration     `json:"grace,omitempty"` // how long a stopped command gets before a SIGKILL; zero is DefaultCommandGrace
	Out   io.Writer         `json:"-"`               // nil discards the output
}

// CommandError is how a Command ended when it did not end on its own with status 0. It unwraps
// to what stopped the command, so errors.Is(err, context.Canceled) works.
type CommandError struct {
	ExitCode int   // -1 if the command never started or died of a signal
	Stopped  error // context.Canceled or context.DeadlineExceeded if it was stopped, nil if it ended on its own
	Killed   bool  // it was sent a SIGKILL: forced, or its grace period ran out
	Err      error // why it could not start, or what waiting on it reported
}

func (e *CommandError) Error() string {
	if err := e.Unwrap(); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit status %d", e.ExitCode)
}

func (e *CommandError) Unwrap() error {
	if e.Stopped != nil {
		return e.Stopped
	}
	return e.Err
}

func (e *CommandError) succeeded() bool {
	return e.Stopped == nil && e.ExitCode == 0
}

// Run returns nil when the command ended on its own with status 0, a *CommandError otherwise.
// A job function with bookkeeping of its own calls it on a copy with the Out it wants.
func (c Command) Run(jc job.Context) error {
	if end := c.execute(jc); !end.succeeded() {
		return end
	}
	return nil
}

func (c Command) execute(jc job.Context) *CommandError {
	ctx := jc.Context()
	if len(c.Argv) == 0 {
		return &CommandError{ExitCode: -1, Err: errors.InvalidArgument.Newf("no command to run")}
	}
	if err := ctx.Err(); err != nil {
		return &CommandError{ExitCode: -1, Stopped: err}
	}
	grace := c.Grace
	if grace <= 0 {
		grace = DefaultCommandGrace
	}
	out := c.Out
	if out == nil {
		out = io.Discard
	}
	p := cmdutil.New(c.Argv[0], c.Argv[1:],
		cmdutil.WithDir(c.Dir),
		cmdutil.WithEnv(commandEnv(c.Env)...),
		cmdutil.WithStdout(out),
		cmdutil.WithStderr(out),
		cmdutil.WithWaitDelay(grace), // so a child still holding the output does not hold up the end
		cmdutil.WithProcessGroup(),
	)
	if err := p.Start(); err != nil {
		return &CommandError{ExitCode: -1, Err: err}
	}
	exited := make(chan error, 1)
	go func() { exited <- p.Wait() }()

	end := &CommandError{}
	stop, kill := ctx.Done(), jc.Signaled(job.SignalKill)
	var graceUp <-chan time.Time
	for {
		select {
		case err := <-exited:
			if end.Stopped != nil {
				// what ignored the SIGTERM goes too; a command that ended on its own keeps its children
				_ = p.Kill()
			}
			end.ExitCode, end.Err = p.ExitCode(), err
			return end
		case <-stop:
			stop = nil
			end.Stopped = ctx.Err()
			_ = p.Terminate()
			timer := time.NewTimer(grace)
			defer timer.Stop()
			graceUp = timer.C
		case <-graceUp:
			graceUp = nil
			end.Killed = true
			_ = p.Kill()
		case <-kill:
			kill = nil
			if end.Stopped == nil {
				end.Stopped = context.Canceled
			}
			end.Killed = true
			_ = p.Kill()
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
