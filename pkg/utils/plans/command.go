// Package plans holds the predefined kinds of plan: for each, what one run of such a plan does,
// for the planner to run inside a run's bookkeeping.
package plans

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

// Command is a plan of entity.PlanKindCommand: each run is the plan's jobs.Command.
type Command struct {
	cmd     jobs.Command
	timeout time.Duration
}

// NewCommand reads a command plan, whose Params is a jobs.Command; its Out is the planner's,
// set for each run.
func NewCommand(plan *entity.Plan) (Command, error) {
	cmd, ok := plan.Params.(jobs.Command)
	if !ok {
		return Command{}, errors.InvalidArgument.Newf("plan %s: a %s plan takes a jobs.Command in Plan.Params, not %T", plan.ID, plan.Kind, plan.Params)
	}
	if len(cmd.Argv) == 0 {
		return Command{}, errors.InvalidArgument.Newf("plan %s has no command", plan.ID)
	}
	if cmd.Out != nil {
		return Command{}, errors.InvalidArgument.Newf("plan %s: a command plan's Out is the planner's, set for each run", plan.ID)
	}
	return Command{cmd: cmd, timeout: plan.Task.Timeout}, nil
}

// Header heads each run's log: the command line.
func (c Command) Header() string {
	return strings.Join(c.cmd.Argv, " ")
}

// Run runs the command once, writing its output to out, and says how the run ended. Its error is
// the command's.
func (c Command) Run(jc job.Context, out io.Writer) (Outcome, error) {
	cmd := c.cmd // this run's copy
	cmd.Out = out
	err := cmd.Run(jc)
	return commandOutcome(err, c.timeout), err
}

func commandOutcome(err error, timeout time.Duration) Outcome {
	if err == nil {
		return Outcome{State: entity.PlanRunSucceeded}
	}
	var failed *jobs.CommandError
	if !errors.As(err, &failed) {
		return Outcome{State: entity.PlanRunFailed, Error: err.Error(), ExitCode: -1}
	}
	o := Outcome{State: entity.PlanRunFailed, ExitCode: failed.ExitCode}
	switch {
	case errors.Is(failed.Stopped, context.DeadlineExceeded):
		o.Error = fmt.Sprintf("timed out after %s", timeout)
	case failed.Stopped != nil:
		o.State, o.Error = entity.PlanRunCanceled, "canceled"
	default:
		o.Error = failed.Error() // its exit status, or why it could not start
	}
	return o
}
