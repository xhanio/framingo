package tracing

import (
	"context"

	"github.com/xhanio/errors"
)

// Alive fails only when enabled without a provider (a restart rebuilds it);
// a stopped manager stays alive so dependents don't cascade-restart, and the
// collector is never probed: tracing must not degrade the app.
func (m *manager) Alive(_ context.Context) error {
	m.mu.RLock()
	enabled := m.settings.enabled
	m.mu.RUnlock()
	live, stopped := m.controller.state()
	if enabled && !live && !stopped {
		return errors.Newf("tracing %s is enabled but has no provider", m.Name())
	}
	return nil
}
