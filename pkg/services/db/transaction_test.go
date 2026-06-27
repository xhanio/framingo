package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// newTransactionTestMgr builds a SQLite-backed db.Manager with the connection
// pool capped at maxOpen, then creates a single "items" table for the tests
// below. Capping max_open=1 reproduces the deadlock condition that motivated
// the nested-transaction reuse change in context.go.
func newTransactionTestMgr(t *testing.T, maxOpen int) db.Manager {
	t.Helper()

	v := viper.New()
	v.Set("db.connection.max_open", maxOpen)
	v.Set("db.connection.max_idle", maxOpen)
	v.Set("db.connection.exec_timeout", 2*time.Second)
	ctx := confutil.WrapContext(context.Background(), v)

	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}),
	)
	require.NoError(t, mgr.Init(ctx))

	require.NoError(t, mgr.ORM().Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)`).Error)
	return mgr
}

func countItems(t *testing.T, mgr db.Manager) int64 {
	t.Helper()
	var n int64
	require.NoError(t, mgr.ORM().Raw(`SELECT COUNT(*) FROM items`).Scan(&n).Error)
	return n
}

func TestTransaction_NestedCallReusesOuterConnection(t *testing.T) {
	mgr := newTransactionTestMgr(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := mgr.Transaction(ctx, func(outerCtx context.Context) error {
		if err := mgr.FromContext(outerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "outer").Error; err != nil {
			return err
		}
		return mgr.Transaction(outerCtx, func(innerCtx context.Context) error {
			return mgr.FromContext(innerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "inner").Error
		})
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), countItems(t, mgr))
}

func TestTransaction_NestedInnerErrorRollsBackInnerOnly(t *testing.T) {
	mgr := newTransactionTestMgr(t, 1)

	err := mgr.Transaction(context.Background(), func(outerCtx context.Context) error {
		if err := mgr.FromContext(outerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "outer").Error; err != nil {
			return err
		}
		innerErr := mgr.Transaction(outerCtx, func(innerCtx context.Context) error {
			if err := mgr.FromContext(innerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "inner").Error; err != nil {
				return err
			}
			return fmt.Errorf("force inner rollback")
		})
		require.Error(t, innerErr)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), countItems(t, mgr))
}

func TestTransaction_NestedOuterErrorRollsBackEverything(t *testing.T) {
	mgr := newTransactionTestMgr(t, 1)

	sentinel := fmt.Errorf("force outer rollback")
	err := mgr.Transaction(context.Background(), func(outerCtx context.Context) error {
		if err := mgr.FromContext(outerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "outer").Error; err != nil {
			return err
		}
		if err := mgr.Transaction(outerCtx, func(innerCtx context.Context) error {
			return mgr.FromContext(innerCtx).Exec(`INSERT INTO items(name) VALUES (?)`, "inner").Error
		}); err != nil {
			return err
		}
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, int64(0), countItems(t, mgr))
}

func TestTransaction_RootCallCommitsAndRollsBackUnchanged(t *testing.T) {
	mgr := newTransactionTestMgr(t, 1)

	require.NoError(t, mgr.Transaction(context.Background(), func(ctx context.Context) error {
		return mgr.FromContext(ctx).Exec(`INSERT INTO items(name) VALUES (?)`, "committed").Error
	}))
	assert.Equal(t, int64(1), countItems(t, mgr))

	sentinel := fmt.Errorf("rollback root")
	err := mgr.Transaction(context.Background(), func(ctx context.Context) error {
		if err := mgr.FromContext(ctx).Exec(`INSERT INTO items(name) VALUES (?)`, "rolled-back").Error; err != nil {
			return err
		}
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, int64(1), countItems(t, mgr))
}
