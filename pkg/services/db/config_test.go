package db_test

import (
	"context"
	"path/filepath"
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
