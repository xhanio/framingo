package planner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Start(ctx context.Context) error {
	err := m.tm.Start(ctx)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	var errs []error
	errs = append(errs, m.tm.Stop(wait))
	return errors.Combine(errs...)
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
