package planner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/task"
)

const commandKillWait = 2 * time.Second

// commandJob is the job of a command plan: each run is a command task built from the plan's
// CommandParams, with the plan's grace period, and its log is headed by the command line.
func (m *manager) commandJob(plan *entity.Plan) (job.Func, error) {
	params, ok := plan.Params.(entity.CommandParams)
	if !ok {
		return nil, errors.InvalidArgument.Newf("plan %s: a %s plan takes entity.CommandParams in Plan.Params, not %T", plan.ID, plan.Kind, plan.Params)
	}
	if len(params.Command) == 0 {
		return nil, errors.InvalidArgument.Newf("plan %s has no command", plan.ID)
	}
	cmd := task.Command{Argv: params.Command, Dir: params.Dir, Env: params.Env, Grace: plan.Task.Grace()}
	timeout := plan.Task.Timeout
	return m.runJob(plan, strings.Join(cmd.Argv, " "), func(jc job.Context, out io.Writer) (outcome, error) {
		c := cmd // this run's copy
		c.Out = out
		err := c.Run(jc)
		return commandOutcome(err, timeout), err
	}), nil
}

func commandOutcome(err error, timeout time.Duration) outcome {
	var res task.CommandResult // a success: status 0, not stopped
	var failed *task.CommandError
	if errors.As(err, &failed) {
		res = failed.Result
	}
	o := outcome{ExitCode: res.ExitCode}
	switch {
	case errors.Is(res.Stopped, context.DeadlineExceeded):
		o.State, o.Error = entity.PlanRunFailed, fmt.Sprintf("timed out after %s", timeout)
	case res.Stopped != nil:
		o.State, o.Error = entity.PlanRunCanceled, "canceled"
	case res.ExitCode == 0:
		o.State = entity.PlanRunSucceeded
	case res.ExitCode > 0:
		// the output is already in the tail and the log, the error need not repeat it
		o.State, o.Error = entity.PlanRunFailed, fmt.Sprintf("exit status %d", res.ExitCode)
	default:
		o.State, o.Error = entity.PlanRunFailed, res.Err.Error()
	}
	return o
}
