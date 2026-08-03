package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
)

// fakeDB implements model.Database over a hand-built *sql.DB whose
// connector either hands out connections or refuses them, which is all
// PingContext exercises.
type fakeDB struct {
	db *sql.DB
}

func (f *fakeDB) Name() string                             { return "fake/db" }
func (f *fakeDB) Dependencies() []common.Service           { return nil }
func (f *fakeDB) ORM() *gorm.DB                            { return nil }
func (f *fakeDB) DB() *sql.DB                              { return f.db }
func (f *fakeDB) FromContext(ctx context.Context) *gorm.DB { return nil }
func (f *fakeDB) FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc) {
	return nil, func() {}
}
func (f *fakeDB) Cleanup(schema bool) error { return nil }
func (f *fakeDB) Reload() error             { return nil }
func (f *fakeDB) Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error {
	return fn(ctx)
}

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

func newFakeDB(connectErr error) *fakeDB {
	return &fakeDB{db: sql.OpenDB(stubConnector{err: connectErr})}
}

func TestReadyPingsDatabase(t *testing.T) {
	m := newManager(newFakeDB(nil))
	require.NoError(t, m.Ready())

	m = newManager(newFakeDB(sql.ErrConnDone))
	err := m.Ready()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping")
}

func TestReadyWithoutHandle(t *testing.T) {
	m := newManager(&fakeDB{db: nil})
	require.Error(t, m.Ready())
}

func TestAliveChecksOwnWiringOnly(t *testing.T) {
	// Wired: alive, trivially.
	m := newManager(newFakeDB(nil))
	require.NoError(t, m.Alive())

	// An unreachable database must NOT fail liveness: the supervisor restarts
	// a service whose Alive fails, and no repository restart fixes a database
	// outage. That is Ready's story.
	m = newManager(newFakeDB(sql.ErrConnDone))
	assert.NoError(t, m.Alive())

	// A missing handle is the repository's own wiring being broken.
	m = newManager(&fakeDB{db: nil})
	require.Error(t, m.Alive())
}
