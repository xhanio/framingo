// Package testutil holds test helpers shared across framingo's own packages
// and usable by apps built on it: a SQLite-backed db.Manager, and doubles for
// the model interfaces a component receives.
package testutil

import (
	"context"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// SetupDB returns an initialized db.Manager backed by an in-memory SQLite
// database, so a test exercises real SQL rather than a repository double.
// Requires CGO_ENABLED=1, which the sqlite driver needs.
func SetupDB() (db.Manager, error) {
	m := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{}), // empty DBName → :memory: DSN
		db.WithLogger(log.New(log.WithLevel(-1))),
	)
	if err := m.Init(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to init db")
	}
	return m, nil
}
