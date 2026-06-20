package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func SetupDB() (db.Manager, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get work directory")
	}
	parts := strings.Split(wd, "/pkg")
	if len(parts) <= 1 {
		return nil, errors.Newf("failed to locate package path")
	}

	// Create a new instance of the manager type.
	m := db.New(
		db.WithType("postgres"),
		db.WithDataSource(db.Source{
			Host:     "localhost",
			Port:     5431,
			User:     "test",
			Password: "test",
			DBName:   "testdb",
		}),
		db.WithLogger(log.New(log.WithLevel(-1))),
		db.WithMigration(filepath.Join(parts[0], "env/default/config/demo/sql"), 000000),
	)
	if err := m.Init(context.Background()); err != nil {
		return nil, errors.Wrapf(err, "failed to init db")
	}
	return m, nil
}
