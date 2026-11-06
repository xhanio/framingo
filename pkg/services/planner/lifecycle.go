package planner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xhanio/framingo/pkg/utils/printutil"
)

// Start has nothing to do: the task manager is the caller's, a dependency that starts first.
func (m *manager) Start(ctx context.Context) error {
	return nil
}

// Stop ends and forgets the planner's own plans, leaving the task manager and whatever else
// runs on it alone.
func (m *manager) Stop(wait bool) error {
	m.Lock()
	defer m.Unlock()
	for id, todo := range m.todos {
		m.tm.Remove(todo.Task)
		delete(m.todos, id)
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
