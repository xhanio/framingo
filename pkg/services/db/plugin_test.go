package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
)

// countingPlugin is a real gorm.Plugin rather than a flag the test flips: it
// registers a raw callback, so what the assertions observe is gorm dispatching
// through the installed plugin, not the manager reporting that it installed
// one.
type countingPlugin struct {
	statements int
}

func (p *countingPlugin) Name() string { return "counting" }

func (p *countingPlugin) Initialize(g *gorm.DB) error {
	return g.Callback().Raw().After("gorm:raw").Register("counting:raw", func(tx *gorm.DB) {
		p.statements++
	})
}

func TestWithPluginsInstallsPluginOnConnect(t *testing.T) {
	p := &countingPlugin{}
	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}),
		db.WithPlugins(p),
	)
	require.NoError(t, mgr.Init(context.Background()))

	require.NoError(t, mgr.ORM().Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`).Error)
	require.NoError(t, mgr.ORM().Exec(`INSERT INTO items (name) VALUES ('a')`).Error)

	assert.Equal(t, 2, p.statements)
}

// brokenPlugin fails to install. A plugin that cannot register its callbacks
// is silent misconfiguration otherwise: queries would run untraced with
// nothing in the logs to say why.
type brokenPlugin struct{}

func (brokenPlugin) Name() string { return "broken" }

func (brokenPlugin) Initialize(*gorm.DB) error {
	return errors.Newf("cannot install")
}

func TestPluginInstallFailureFailsInit(t *testing.T) {
	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}),
		db.WithPlugins(brokenPlugin{}),
	)
	err := mgr.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
	assert.Contains(t, err.Error(), "cannot install")
}

// WithPlugins keeps a slice and does not dedup by Name(), because gorm already
// does: DB.Use rejects a second plugin registered under a name it has seen,
// with ErrRegistered. So a double-wired plugin fails Init loudly, naming the
// plugin. Deduping in WithPlugins would swallow that wiring bug instead -
// silently installing one of the two and leaving no trace of the other.
// Characterization: this pins gorm's behavior, which the decision rests on.
func TestDuplicatePluginNameFailsInit(t *testing.T) {
	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}),
		db.WithPlugins(&countingPlugin{}),
		db.WithPlugins(&countingPlugin{}), // same Name(), different instance
	)
	err := mgr.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "counting")
	assert.ErrorIs(t, err, gorm.ErrRegistered)
}
