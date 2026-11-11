package plan

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
	"github.com/xhanio/framingo/pkg/utils/plans"
)

var (
	_ common.Message = entity.PlanRunStarted{}
	_ common.Message = entity.PlanRunFinished{}
)

// run is one run of a plan of a kind, from its start to its end: its record, its log file and
// the tail of its output.
type run struct {
	entity.PlanRun
	reg  *registration
	log  *os.File
	tail ioutil.TailWriter
}

// runJob is the job of a plan of a kind: every execution of it is a run doing body.
func (m *manager) runJob(reg *registration, header string, body func(job.Context, io.Writer) (plans.Outcome, error)) job.Func {
	return func(jc job.Context) error {
		r := m.startRun(reg, jc, header)
		o, err := body(jc, r.out())
		m.endRun(r, o)
		if err != nil {
			return errors.Wrapf(err, "plan %s run %s", reg.plan.ID, r.ID)
		}
		return nil
	}
}

// startRun begins a run of reg's plan. A firing of Trigger gives its source to all its runs and
// the ID Trigger returned to the first, as retries and requeues are runs of their own.
func (m *manager) startRun(reg *registration, jc job.Context, header string) *run {
	r := &run{
		PlanRun: entity.PlanRun{
			ID:        uuid.NewString(),
			PlanID:    reg.plan.ID,
			Source:    entity.PlanRunSchedule,
			State:     entity.PlanRunRunning,
			StartedAt: time.Now(),
		},
		reg:  reg,
		tail: ioutil.NewTailWriter(m.tailBytes),
	}
	if tr, ok := jc.Params().(*triggered); ok {
		r.Source = tr.source
		if tr.claim() {
			r.ID = tr.id
		}
	}
	r.log = m.openRunLog(r, header)
	reg.record(r.PlanRun)
	m.report(entity.PlanRunStarted{PlanRun: r.PlanRun})
	return r
}

// out is where the run's output goes: its log file, if it has one, and its tail.
func (r *run) out() io.Writer {
	if r.log == nil {
		return r.tail
	}
	return io.MultiWriter(r.log, r.tail)
}

func (m *manager) endRun(r *run, o plans.Outcome) {
	if r.log != nil {
		_ = r.log.Close()
	}
	r.EndedAt = time.Now()
	r.State, r.Error, r.ExitCode = o.State, o.Error, o.ExitCode
	r.OutputTail = r.tail.String()
	r.reg.record(r.PlanRun)
	m.report(entity.PlanRunFinished{PlanRun: r.PlanRun})
}

// openRunLog creates the run's log file and records its path; a run without one still runs.
func (m *manager) openRunLog(r *run, header string) *os.File {
	if m.logDir == "" {
		return nil
	}
	path := filepath.Join(m.logDir, r.PlanID, r.StartedAt.Format("20060102-150405")+"-"+r.ID[:8]+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.log.Warnf("plan %s run %s: cannot create log dir: %v", r.PlanID, r.ID, err)
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		m.log.Warnf("plan %s run %s: cannot create log file: %v", r.PlanID, r.ID, err)
		return nil
	}
	fmt.Fprintf(f, "# plan %s run %s (%s) at %s\n# %s\n\n", r.PlanID, r.ID, r.Source,
		r.StartedAt.Format(time.RFC3339), header)
	r.LogPath = path
	return f
}

// endUnstarted ends the run Trigger promised, if its firing was dropped before any run of it
// started: that run is reported by PlanRunFinished alone.
func (m *manager) endUnstarted(reg *registration) {
	tr := reg.unstarted()
	if tr == nil {
		return
	}
	run := entity.PlanRun{
		ID:        tr.id,
		PlanID:    reg.plan.ID,
		Source:    tr.source,
		State:     entity.PlanRunCanceled,
		StartedAt: tr.at,
		EndedAt:   time.Now(),
		ExitCode:  -1,
		Error:     "canceled before it started",
	}
	reg.record(run)
	m.report(entity.PlanRunFinished{PlanRun: run})
}

func (m *manager) report(msg common.Message) {
	if m.ms != nil {
		m.ms.SendMessage(context.Background(), m, msg)
	}
}
