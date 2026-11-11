package planner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

// The task manager is the caller's, a dependency started before the planner.
func (m *manager) Start(ctx context.Context) error {
	return nil
}

// Stop forgets the plans and waits for their runs until ctx ends, then sends what still runs
// SignalKill. It leaves the task manager running: other tasks may share it.
func (m *manager) Stop(ctx context.Context) error {
	m.Lock()
	tasks := make([]*entity.Task, 0, len(m.plans))
	for _, plan := range m.plans {
		tasks = append(tasks, plan.Task)
	}
	clear(m.plans)
	m.Unlock()
	if err := m.tm.Remove(ctx, tasks...); err != nil {
		return errors.Wrapf(err, "planner stopped with runs still going")
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
