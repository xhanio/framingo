package messagebus

import (
	"context"
	"io"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	// until Stop: Start's ctx bounds the start alone
	ctx, m.cancel = context.WithCancel(context.WithoutCancel(ctx))
	m.ctx = ctx
	m.started = true
	modules := make([]common.Named, 0, len(m.modules))
	for _, mod := range m.modules {
		modules = append(modules, mod)
	}
	m.mu.Unlock()

	for _, mod := range modules {
		if err := m.subscribe(ctx, mod); err != nil {
			m.log.Error(err)
		}
	}
	return nil
}

// Stop waits for handlers until ctx ends; one ignoring its canceled context is left running.
func (m *manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	started := m.started
	m.started = false
	cancel := m.cancel
	modules := make([]string, 0, len(m.modules))
	for name := range m.modules {
		modules = append(modules, name)
	}
	m.mu.Unlock()

	if started {
		// closes their channels so listen() exits
		for _, name := range modules {
			if err := m.bus.Unsubscribe(name, m.topic); err != nil {
				m.log.Errorf("failed to unsubscribe %s: %v", name, err)
			}
		}
		cancel()
	}
	if err := m.wg.Wait(ctx); err != nil {
		return errors.Wrapf(err, "%s stopped with handlers still going", m.name)
	}
	return nil
}

// Reconcile follows pubsub: a start has forgotten every subscription, so each
// module subscribes again. Its listener ended when the stop closed its channel.
func (m *manager) Reconcile(_ context.Context, dep common.Service, state common.DependencyState) error {
	if dep.Name() != m.bus.Name() || state != common.DependencyRunning {
		return nil
	}
	m.mu.Lock()
	started, ctx := m.started, m.ctx
	modules := make([]common.Named, 0, len(m.modules))
	for _, mod := range m.modules {
		modules = append(modules, mod)
	}
	m.mu.Unlock()
	if !started {
		return nil
	}
	var errs []error
	for _, mod := range modules {
		errs = append(errs, m.subscribe(ctx, mod))
	}
	return errors.Combine(errs...)
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
