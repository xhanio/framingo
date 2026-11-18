package testutil

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type row struct {
	ID int
}

// every connection in the pool sees the same database: concurrent queries
// beyond one connection still find the schema
func TestSetupDBSharesOneDatabaseAcrossConnections(t *testing.T) {
	m, err := SetupDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Cleanup(true)) })
	require.NoError(t, m.ORM().AutoMigrate(&row{}))

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var n int64
			errs <- m.FromContext(ctx).Model(&row{}).Count(&n).Error
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestSetupDBGivesEachCallItsOwnDatabase(t *testing.T) {
	a, err := SetupDB()
	require.NoError(t, err)
	b, err := SetupDB()
	require.NoError(t, err)
	require.NoError(t, a.ORM().AutoMigrate(&row{}))
	require.False(t, b.ORM().Migrator().HasTable(&row{}), "another call's table")
}
