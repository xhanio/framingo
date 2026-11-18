// Package testutil holds test helpers and model doubles for framingo and apps built on it.
package testutil

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/utils/log"
)

var dbSeq atomic.Int64

// SetupDB uses in-memory SQLite and requires CGO_ENABLED=1. Each call gets a
// database of its own, shared by every connection in its pool: a plain
// :memory: gives each connection a separate, empty database.
func SetupDB() (db.Manager, error) {
	m := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{
			DBName: fmt.Sprintf("file:testutil-%d", dbSeq.Add(1)),
			Params: map[string]string{"mode": "memory", "cache": "shared"},
		}),
		db.WithLogger(log.New(log.WithLevel(-1))),
	)
	if err := m.Init(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to init db")
	}
	return m, nil
}
