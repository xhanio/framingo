package pubsub

import (
	"context"
	"fmt"
	"io"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if err := m.bus.Start(ctx); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	if err := m.bus.Stop(wait); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("stat", "value")
	t.Row("backend", fmt.Sprintf("%T", m.bus))
	t.Row("published", m.published.Load())
	t.NewLine()
	t.Flush()
}
