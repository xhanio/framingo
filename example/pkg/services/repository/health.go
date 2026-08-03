package repository

import (
	"context"
	"time"

	"github.com/xhanio/errors"
)

// healthCheckTimeout bounds the readiness ping so a stalled database cannot
// wedge the supervisor's monitor loop.
const healthCheckTimeout = 3 * time.Second

// Alive implements common.Liveness for the repository's own wiring only. A
// liveness failure makes the supervisor restart this service, and no
// repository restart fixes an unreachable database - that is Ready's story.
func (m *manager) Alive(_ context.Context) error {
	if m.db == nil || m.db.DB() == nil {
		return errors.Newf("repository has no database handle")
	}
	return nil
}

// Ready implements common.Readiness by pinging the database: "not ready"
// means queries will fail right now. The supervisor reports it and rolls it
// up into every dependent service's healthcheck without restarting anything.
func (m *manager) Ready(ctx context.Context) error {
	if m.db == nil || m.db.DB() == nil {
		return errors.Newf("repository has no database handle")
	}
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if err := m.db.DB().PingContext(ctx); err != nil {
		return errors.Wrapf(err, "database ping failed")
	}
	return nil
}
