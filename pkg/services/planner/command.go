package planner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/cmdutil"
	"github.com/xhanio/framingo/pkg/utils/ioutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

const commandWaitDelay = 10 * time.Second

var (
	_ common.Message = entity.PlanRunStarted{}
	_ common.Message = entity.PlanRunFinished{}
)

// triggeredRun is what Trigger gives the firing it starts: every run of it takes the source,
// the first takes the ID Trigger returned, and the retries and requeues after it, being runs
// of their own, take new ones.
type triggeredRun struct {
	id      string
	source  entity.PlanRunSource
	claimed atomic.Bool
}

func (m *manager) buildJob(plan *entity.Plan) error {
	if plan.Task == nil {
		return errors.InvalidArgument.Newf("plan %s has no task", plan.ID)
	}
	switch plan.Kind {
	case entity.PlanKindCommand:
		params, ok := plan.Task.Params.(entity.CommandParams)
		if !ok {
			return errors.InvalidArgument.Newf("plan %s: a %s plan takes entity.CommandParams, not %T", plan.ID, plan.Kind, plan.Task.Params)
		}
		if len(params.Command) == 0 {
			return errors.InvalidArgument.Newf("plan %s has no command", plan.ID)
		}
		plan.Task.Job = job.New(plan.ID, func(jc job.Context) error {
			return m.runCommand(jc, plan, params)
		}, job.WithLabels(plan.Metadata), job.WithLogger(m.log))
		return nil
	default:
		return errors.InvalidArgument.Newf("plan %s has an unknown kind %q", plan.ID, plan.Kind)
	}
}

func (m *manager) runCommand(jc job.Context, plan *entity.Plan, params entity.CommandParams) error {
	ctx := jc.Context()
	run := entity.PlanRun{
		ID:        uuid.NewString(),
		PlanID:    plan.ID,
		Source:    entity.PlanRunSchedule,
		State:     entity.PlanRunRunning,
		StartedAt: time.Now(),
	}
	if triggered, ok := jc.GetParams().(*triggeredRun); ok {
		run.Source = triggered.source
		if triggered.claimed.CompareAndSwap(false, true) {
			run.ID = triggered.id
		}
	}
	tail := ioutil.NewTailWriter(m.tailBytes)
	var out io.Writer = tail
	if f := m.createRunLog(&run, params); f != nil {
		defer f.Close()
		out = io.MultiWriter(f, tail)
	}
	jc.SetResult(run)
	m.report(entity.PlanRunStarted{PlanRun: run})

	cmd := cmdutil.New(params.Command[0], params.Command[1:],
		cmdutil.WithContext(ctx),
		cmdutil.WithDir(params.Dir),
		cmdutil.WithEnv(commandEnv(params.Env)...),
		cmdutil.WithProcessGroup(),
		cmdutil.WithWaitDelay(commandWaitDelay),
		cmdutil.WithMaxBuffer(m.tailBytes),
		cmdutil.WithStdout(out),
		cmdutil.WithStderr(out),
	)
	err := cmd.Start()

	run.EndedAt = time.Now()
	run.ExitCode = cmd.ExitCode()
	run.OutputTail = tail.String()
	switch {
	case err == nil:
		run.State = entity.PlanRunSucceeded
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		run.State = entity.PlanRunFailed
		run.Error = fmt.Sprintf("timed out after %s", plan.Task.Timeout)
	case ctx.Err() != nil:
		run.State = entity.PlanRunCanceled
		run.Error = "canceled"
	case run.ExitCode > 0:
		// the output is already in the tail and the log, the error need not repeat it
		run.State = entity.PlanRunFailed
		run.Error = fmt.Sprintf("exit status %d", run.ExitCode)
	default:
		run.State = entity.PlanRunFailed
		run.Error = err.Error()
	}
	jc.SetResult(run)
	m.report(entity.PlanRunFinished{PlanRun: run})
	if err != nil {
		return errors.Wrapf(err, "plan %s run %s", plan.ID, run.ID)
	}
	return nil
}

// createRunLog opens the run's log file and records its path; a run without one still runs.
func (m *manager) createRunLog(run *entity.PlanRun, params entity.CommandParams) *os.File {
	if m.logDir == "" {
		return nil
	}
	path := filepath.Join(m.logDir, run.PlanID, run.StartedAt.Format("20060102-150405")+"-"+run.ID[:8]+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.log.Warnf("plan %s run %s: cannot create log dir: %v", run.PlanID, run.ID, err)
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		m.log.Warnf("plan %s run %s: cannot create log file: %v", run.PlanID, run.ID, err)
		return nil
	}
	fmt.Fprintf(f, "# plan %s run %s (%s) at %s\n# %s\n\n", run.PlanID, run.ID, run.Source,
		run.StartedAt.Format(time.RFC3339), strings.Join(params.Command, " "))
	run.LogPath = path
	return f
}

func commandEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func (m *manager) report(msg common.Message) {
	if m.es != nil {
		m.es.SendMessage(context.Background(), m, msg)
	}
}
