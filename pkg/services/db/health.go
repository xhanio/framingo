package db

import (
	"context"
	"time"

	"github.com/xhanio/errors"
)

// keeps a stalled database from wedging the supervisor's monitor
const healthCheckTimeout = 3 * time.Second

// Alive fails only on a missing handle (a restart reconnects); an unreachable
// database is Ready's concern, not liveness.
func (m *manager) Alive(_ context.Context) error {
	if m.sqlDB == nil {
		return errors.Newf("database %s has no connection handle", m.Name())
	}
	return nil
}

func (m *manager) Ready(ctx context.Context) error {
	if m.sqlDB == nil {
		return errors.Newf("database %s has no connection handle", m.Name())
	}
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if err := m.sqlDB.PingContext(ctx); err != nil {
		return errors.Wrapf(err, "database ping failed")
	}
	return nil
}
