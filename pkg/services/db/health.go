package db

import (
	"context"
	"time"

	"github.com/xhanio/errors"
)

// healthCheckTimeout bounds the readiness ping so a stalled database cannot
// wedge the supervisor's monitor loop.
const healthCheckTimeout = 3 * time.Second

// Alive implements common.Liveness for the manager's own wiring only: a
// missing handle means Init never connected, and Init reconnects, so a
// restart is exactly the remedy. An unreachable database must not fail
// liveness - restarting this client does not raise a database server; that
// is Ready's story.
func (m *manager) Alive() error {
	if m.sqlDB == nil {
		return errors.Newf("database %s has no connection handle", m.Name())
	}
	return nil
}

// Ready implements common.Readiness by pinging the database: "not ready"
// means queries will fail right now. The supervisor reports it and rolls it
// up into every dependent service's healthcheck without restarting anything.
func (m *manager) Ready() error {
	if m.sqlDB == nil {
		return errors.Newf("database %s has no connection handle", m.Name())
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	if err := m.sqlDB.PingContext(ctx); err != nil {
		return errors.Wrapf(err, "database ping failed")
	}
	return nil
}
