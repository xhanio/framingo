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
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

// commandJob is the job of a command plan: each run is the plan's jobs.Command, and its log is
// headed by the command line.
func (m *manager) commandJob(plan *entity.Plan) (job.Func, error) {
	cmd, ok := plan.Params.(jobs.Command)
	if !ok {
		return nil, errors.InvalidArgument.Newf("plan %s: a %s plan takes a jobs.Command in Plan.Params, not %T", plan.ID, plan.Kind, plan.Params)
	}
	if len(cmd.Argv) == 0 {
		return nil, errors.InvalidArgument.Newf("plan %s has no command", plan.ID)
	}
	if cmd.Out != nil {
		return nil, errors.InvalidArgument.Newf("plan %s: a command plan's Out is the planner's, set for each run", plan.ID)
	}
	timeout := plan.Task.Timeout
	return m.runJob(plan, strings.Join(cmd.Argv, " "), func(jc job.Context, out io.Writer) (outcome, error) {
		c := cmd // this run's copy
		c.Out = out
		err := c.Run(jc)
		return commandOutcome(err, timeout), err
	}), nil
}

func commandOutcome(err error, timeout time.Duration) outcome {
	if err == nil {
		return outcome{State: entity.PlanRunSucceeded}
	}
	var failed *jobs.CommandError
	if !errors.As(err, &failed) {
		return outcome{State: entity.PlanRunFailed, Error: err.Error(), ExitCode: -1}
	}
	o := outcome{State: entity.PlanRunFailed, ExitCode: failed.ExitCode}
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
