package messagebus

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true
	modules := make([]common.Named, 0, len(m.modules))
	for _, mod := range m.modules {
		modules = append(modules, mod)
	}
	m.mu.Unlock()

	for _, mod := range modules {
		m.subscribe(mod)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	cancel := m.cancel
	modules := make([]string, 0, len(m.modules))
	for name := range m.modules {
		modules = append(modules, name)
	}
	m.mu.Unlock()

	// Unsubscribe each module — closes their channels so listen() exits.
	for _, name := range modules {
		if err := m.bus.Unsubscribe(name, m.topic); err != nil {
			m.log.Errorf("failed to unsubscribe %s: %v", name, err)
		}
	}
	if cancel != nil {
		cancel()
	}
	if wait {
		m.wg.Wait()
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("stat", "value")
	t.Row("topic", m.topic)
	m.mu.Lock()
	t.Row("modules", len(m.modules))
	if debug {
		for name := range m.modules {
			t.Row("module", name)
		}
	}
	m.mu.Unlock()
	t.NewLine()
	t.Flush()
}
