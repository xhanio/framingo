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
	"github.com/xhanio/framingo/pkg/utils/ioutil"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/task"
)

const commandKillWait = 2 * time.Second

var (
	_ common.Message = entity.PlanRunStarted{}
	_ common.Message = entity.PlanRunFinished{}
)

// triggeredRun goes with a firing of Trigger: all its runs take the source, only the first
// the ID Trigger returned, as retries and requeues are runs of their own.
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
		cmd := task.Command{Argv: params.Command, Dir: params.Dir, Env: params.Env, Grace: plan.Task.Grace()}
		plan.Task.Job = job.New(plan.ID, func(jc job.Context) error {
			return m.runCommand(jc, plan, cmd)
		}, job.WithLabels(plan.Metadata), job.WithLogger(m.log))
		return nil
	default:
		return errors.InvalidArgument.Newf("plan %s has an unknown kind %q", plan.ID, plan.Kind)
	}
}

func (m *manager) runCommand(jc job.Context, plan *entity.Plan, cmd task.Command) error {
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
	if f := m.createRunLog(&run, cmd.Argv); f != nil {
		defer f.Close()
		out = io.MultiWriter(f, tail)
	}
	jc.SetResult(run)
	m.report(entity.PlanRunStarted{PlanRun: run})

	cmd.Out = out // cmd is this run's copy
	err := cmd.Run(jc)
	var res task.CommandResult // a success: status 0, not stopped
	var failed *task.CommandError
	if errors.As(err, &failed) {
		res = failed.Result
	}

	run.EndedAt = time.Now()
	run.ExitCode = res.ExitCode
	run.OutputTail = tail.String()
	run.State, run.Error = runOutcome(res, plan.Task.Timeout)
	jc.SetResult(run)
	m.report(entity.PlanRunFinished{PlanRun: run})
	if err != nil {
		return errors.Wrapf(err, "plan %s run %s", plan.ID, run.ID)
	}
	return nil
}

func runOutcome(res task.CommandResult, timeout time.Duration) (entity.PlanRunState, string) {
	switch {
	case errors.Is(res.Stopped, context.DeadlineExceeded):
		return entity.PlanRunFailed, fmt.Sprintf("timed out after %s", timeout)
	case res.Stopped != nil:
		return entity.PlanRunCanceled, "canceled"
	case res.ExitCode == 0:
		return entity.PlanRunSucceeded, ""
	case res.ExitCode > 0:
		// the output is already in the tail and the log, the error need not repeat it
		return entity.PlanRunFailed, fmt.Sprintf("exit status %d", res.ExitCode)
	default:
		return entity.PlanRunFailed, res.Err.Error()
	}
}

// createRunLog opens the run's log file and records its path; a run without one still runs.
func (m *manager) createRunLog(run *entity.PlanRun, argv []string) *os.File {
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
		run.StartedAt.Format(time.RFC3339), strings.Join(argv, " "))
	run.LogPath = path
	return f
}

// By path, not run ID, since the planner only remembers the latest run. The check is lexical:
// the planner made these files.
func (m *manager) OpenRunLog(path string, tailBytes int64) (io.ReadCloser, error) {
	if m.logDir == "" {
		return nil, errors.NotFound.Newf("failed to open run log %s: no log dir is configured", path)
	}
	root, err := filepath.Abs(m.logDir)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if rel, err := filepath.Rel(root, abs); err != nil || rel == "." || !filepath.IsLocal(rel) {
		return nil, errors.Forbidden.Newf("failed to open run log %s: it is not under the log dir", path)
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NotFound.Newf("failed to open run log %s: the file is gone", path)
		}
		return nil, errors.Wrap(err)
	}
	if tailBytes > 0 {
		if st, err := f.Stat(); err == nil && st.Size() > tailBytes {
			if _, err := f.Seek(st.Size()-tailBytes, io.SeekStart); err != nil {
				f.Close()
				return nil, errors.Wrap(err)
			}
		}
	}
	return f, nil
}

func (m *manager) report(msg common.Message) {
	if m.es != nil {
		m.es.SendMessage(context.Background(), m, msg)
	}
}
