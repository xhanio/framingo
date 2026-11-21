package db_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// sql.DB.Stats exposes the pool settings, showing which config root was read
func newSQLiteMgr(t *testing.T, keys map[string]any, opts ...db.Option) db.Manager {
	t.Helper()
	v := viper.New()
	for k, val := range keys {
		v.Set(k, val)
	}
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))
	mgr := db.New(append(opts,
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{DBName: filepath.Join(t.TempDir(), "test.db")}),
	)...)
	require.NoError(t, mgr.Init(ctx))
	return mgr
}

func TestInitReadsUnderPackageRoot(t *testing.T) {
	mgr := newSQLiteMgr(t, map[string]any{"db.connection.max_open": 2})
	assert.Equal(t, 2, mgr.DB().Stats().MaxOpenConnections)
}

func TestInitReadsUnderNameRoot(t *testing.T) {
	mgr := newSQLiteMgr(t, map[string]any{
		"analytics.connection.max_open": 3,
		"db.connection.max_open":        2,
	}, db.WithName("analytics"))
	assert.Equal(t, "analytics", mgr.Name())
	assert.Equal(t, 3, mgr.DB().Stats().MaxOpenConnections)
}

// WithConnection is the pool's baseline: a key the config sets overrides it at every
// Init, and one a later config drops falls back to it.
func TestTheConfigOverridesTheConnectionOptionKeyByKey(t *testing.T) {
	mgr := newSQLiteMgr(t, nil, db.WithConnection(4, 0, 0, 0, 0))
	assert.Equal(t, 4, mgr.DB().Stats().MaxOpenConnections, "no key: the option holds")

	v := viper.New()
	v.Set("db.connection.max_open", 2)
	require.NoError(t, mgr.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	assert.Equal(t, 2, mgr.DB().Stats().MaxOpenConnections, "the key overrides it")

	require.NoError(t, mgr.Init(confutil.WrapContext(context.Background(), confutil.New(viper.New()))))
	assert.Equal(t, 4, mgr.DB().Stats().MaxOpenConnections, "the key dropped: back to the option")
}

// every Init reconnects - a supervisor restart re-inits - and the pool it
// replaces is closed rather than leaked
func TestReInitClosesThePreviousPool(t *testing.T) {
	mgr := newSQLiteMgr(t, nil)
	first := mgr.DB()
	require.NoError(t, mgr.Init(context.Background()))
	assert.NotSame(t, first, mgr.DB(), "Init reconnects")
	assert.ErrorContains(t, first.Ping(), "database is closed")
	assert.NoError(t, mgr.DB().Ping())
}

// a restart re-inits while dependents keep querying through the same handle:
// what they read is replaced whole, never written under them (run with -race)
func TestReInitUnderQueries(t *testing.T) {
	mgr := newSQLiteMgr(t, nil)
	ctx := context.Background()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				// a query on the pool being replaced may fail; reading the handle must not race
				var n int
				mgr.FromContext(ctx).Raw("SELECT 1").Scan(&n)
				tx, cancel := mgr.FromContextTimeout(ctx, 0)
				tx.Raw("SELECT 1").Scan(&n)
				cancel()
				mgr.Transaction(ctx, func(context.Context) error { return nil }) //nolint:errcheck // see above
				mgr.Ready(ctx)                                                   //nolint:errcheck // see above
			}
		})
	}
	for range 5 {
		require.NoError(t, mgr.Init(ctx))
	}
	close(stop)
	wg.Wait()

	var n int
	require.NoError(t, mgr.FromContext(ctx).Raw("SELECT 1").Scan(&n).Error)
	assert.Equal(t, 1, n, "the last pool serves")
}
