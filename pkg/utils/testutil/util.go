package testutil

import (
	"context"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/utils/log"
)

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
