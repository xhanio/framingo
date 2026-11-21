package plan

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

// The task manager is the caller's, a dependency started before the plan manager.
func (m *manager) Start(ctx context.Context) error {
	return nil
}

// Stop forgets the plans and waits for their runs until ctx ends, then sends what still runs
// SignalKill. It leaves the task manager running: other tasks may share it.
func (m *manager) Stop(ctx context.Context) error {
	m.Lock()
	regs := make([]*registration, 0, len(m.plans))
	tasks := make([]*entity.Task, 0, len(m.plans))
	for _, reg := range m.plans {
		regs = append(regs, reg)
		tasks = append(tasks, reg.plan.Task)
	}
	clear(m.plans)
	m.Unlock()
	err := m.tm.Remove(ctx, tasks...)
	for _, reg := range regs {
		m.endUnstarted(reg)
	}
	if err != nil {
		return errors.Wrapf(err, "plan manager stopped with runs still going")
	}
	return nil
}

// Reconcile follows the task manager: a stop has dropped every firing not yet
// started, which end canceled as at the plan manager's own stop, and a start
// takes every plan's task again.
func (m *manager) Reconcile(_ context.Context, dep common.Service, state common.DependencyState) error {
	if dep.Name() != m.tm.Name() {
		return nil
	}
	switch state {
	case common.DependencyStopped:
		m.RLock()
		regs := make([]*registration, 0, len(m.plans))
		for _, reg := range m.plans {
			regs = append(regs, reg)
		}
		m.RUnlock()
		for _, reg := range regs {
			m.endUnstarted(reg)
		}
	case common.DependencyRunning:
		// locked, as Add is: a plan Delete has taken out is not put back
		m.Lock()
		defer m.Unlock()
		var errs []error
		for _, reg := range m.plans {
			// a task still there was not forgotten
			if err := m.tm.Add(reg.plan.Task); err != nil && !errors.Is(err, errors.Conflict) {
				errs = append(errs, errors.Wrapf(err, "plan %s", reg.plan.ID))
			}
		}
		return errors.Combine(errs...)
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	if debug {
		pt := printutil.NewTable(w)
		pt.Header(m.Name())
		pt.Title("ID", "Schedule", "StartedAt", "State", "Error", "Labels")
		for _, t := range m.stats(debug) {
			start := ""
			if !t.StartedAt.IsZero() {
				start = t.StartedAt.Local().Format(timeFormat)
			}
			if t.ExecutionTime > 0 {
				start += fmt.Sprintf(" (%s)", t.ExecutionTime.Round(time.Millisecond).String())
			}
			state := string(t.State)
			if t.Cooldown > 0 {
				state += fmt.Sprintf(" (cd:%s)", t.Cooldown.Round(time.Second).String())
			}
			pt.Row(t.ID, t.Schedule, start, state, t.Error, t.Labels.String())
		}
		pt.NewLine()
		pt.Flush()
	}
}
