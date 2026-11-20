package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	m.pool.Store(&pool{sql: sql.OpenDB(stubConnector{})})
	require.NoError(t, m.Ready(context.Background()))

	m.pool.Store(&pool{sql: sql.OpenDB(stubConnector{err: sql.ErrConnDone})})
	err := m.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping")
}

func TestAliveChecksOwnWiringOnly(t *testing.T) {
	m := newManager()
	m.pool.Store(&pool{sql: sql.OpenDB(stubConnector{})})
	require.NoError(t, m.Alive(context.Background()))

	// unreachable database: still alive
	m.pool.Store(&pool{sql: sql.OpenDB(stubConnector{err: sql.ErrConnDone})})
	assert.NoError(t, m.Alive(context.Background()))

	// no handle: not alive
	m.pool.Store(nil)
	require.Error(t, m.Alive(context.Background()))
	require.Error(t, m.Ready(context.Background()))
}
