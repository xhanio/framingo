package metrics

import (
	"context"

	"github.com/xhanio/errors"
)

// Alive implements common.Liveness for the manager's own wiring only:
// enabled with no exporter means Init never built one, and Init rebuilds
// it, so a restart is exactly the remedy. A deliberately stopped manager
// stays alive - the monitor probes stopped services through their
// dependents' chains, and a failing Alive there would cascade restarts into
// every dependent. Collector reachability is deliberately probed nowhere:
// metrics are auxiliary, so a collector outage must never degrade the app
// (which is also why there is no Ready at all, and why export errors go to
// Info instead of here).
func (m *manager) Alive(_ context.Context) error {
	m.mu.RLock()
	enabled := m.enabled
	m.mu.RUnlock()
	live, stopped := m.controller.state()
	if enabled && !live && !stopped {
		return errors.Newf("metrics %s is enabled but has no exporter", m.Name())
	}
	return nil
}
