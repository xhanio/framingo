package planner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/ioutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

var (
	_ common.Message = entity.PlanRunStarted{}
	_ common.Message = entity.PlanRunFinished{}
)

// runFunc is the work of one run, between runJob's bookkeeping: it writes its output to out
// and says how the run ended. Its error is the job's.
type runFunc func(jc job.Context, out io.Writer) (outcome, error)

// outcome is how a run ended, as its PlanRun records it.
type outcome struct {
	State    entity.PlanRunState
	Error    string
	ExitCode int
}

// runJob makes a job whose every run does body inside the run's bookkeeping: its PlanRun, a log
// file headed by header, the tail of its output, and reports as it starts and as it ends.
func (m *manager) runJob(plan *entity.Plan, header string, body runFunc) job.Func {
	return func(jc job.Context) error {
		run := newRun(jc, plan)
		tail := ioutil.NewTailWriter(m.tailBytes)
		var out io.Writer = tail
		if f := m.createRunLog(&run, header); f != nil {
			defer f.Close()
			out = io.MultiWriter(f, tail)
		}
		jc.SetResult(run)
		m.report(entity.PlanRunStarted{PlanRun: run})

		o, err := body(jc, out)

		run.EndedAt = time.Now()
		run.State, run.Error, run.ExitCode = o.State, o.Error, o.ExitCode
		run.OutputTail = tail.String()
		jc.SetResult(run)
		m.report(entity.PlanRunFinished{PlanRun: run})
		if err != nil {
			return errors.Wrapf(err, "plan %s run %s", plan.ID, run.ID)
		}
		return nil
	}
}

// newRun begins the PlanRun of this execution. A firing of Trigger gives its source to all its
// runs and the ID Trigger returned to the first, as retries and requeues are runs of their own.
func newRun(jc job.Context, plan *entity.Plan) entity.PlanRun {
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
	return run
}

// createRunLog opens the run's log file and records its path; a run without one still runs.
func (m *manager) createRunLog(run *entity.PlanRun, header string) *os.File {
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
		run.StartedAt.Format(time.RFC3339), header)
	run.LogPath = path
	return f
}

func (m *manager) report(msg common.Message) {
	if m.ms != nil {
		m.ms.SendMessage(context.Background(), m, msg)
	}
}
