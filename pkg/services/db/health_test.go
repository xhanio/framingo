package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubConnector hands out connections or refuses them - all PingContext
// exercises.
type stubConnector struct {
	err error
}

func (c stubConnector) Connect(context.Context) (driver.Conn, error) {
	if c.err != nil {
		return nil, c.err
	}
	return stubConn{}, nil
}

func (c stubConnector) Driver() driver.Driver { return nil }

type stubConn struct{}

func (stubConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (stubConn) Close() error                        { return nil }
func (stubConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func TestReadyPingsDatabase(t *testing.T) {
	m := newManager()
	m.sqlDB = sql.OpenDB(stubConnector{})
	require.NoError(t, m.Ready())

	m.sqlDB = sql.OpenDB(stubConnector{err: sql.ErrConnDone})
	err := m.Ready()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping")
}

func TestAliveChecksOwnWiringOnly(t *testing.T) {
	// Connected handle: alive.
	m := newManager()
	m.sqlDB = sql.OpenDB(stubConnector{})
	require.NoError(t, m.Alive())

	// An unreachable database must NOT fail liveness: restarting the client
	// does not raise a database server, so the futile-restart loop stays off.
	// That is Ready's story.
	m.sqlDB = sql.OpenDB(stubConnector{err: sql.ErrConnDone})
	assert.NoError(t, m.Alive())

	// No handle at all is broken wiring - Init reconnects, so a restart is
	// exactly the remedy.
	m.sqlDB = nil
	require.Error(t, m.Alive())
	require.Error(t, m.Ready())
}
