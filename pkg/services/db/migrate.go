package db

import (
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/xhanio/errors"
)

func (m *manager) migrate(dir string, version uint) error {
	var driver database.Driver
	var err error
	switch m.dbtype {
	case SQLite:
		driver, err = sqlite.WithInstance(m.sqlDB, &sqlite.Config{})
	case Postgres:
		driver, err = postgres.WithInstance(m.sqlDB, &postgres.Config{})
	case MySQL:
		driver, err = mysql.WithInstance(m.sqlDB, &mysql.Config{})
	case Clickhouse:
		driver, err = clickhouse.WithInstance(m.sqlDB, &clickhouse.Config{})
	}
	if err != nil {
		return errors.Wrap(err)
	}
	migrator, err := migrate.NewWithDatabaseInstance(dir, m.source.DBName, driver)
	if err != nil {
		return errors.Wrap(err)
	}
	if version > 0 {
		m.log.Infof("migrating db to version %d", version)
		err = migrator.Migrate(version)
	} else {
		m.log.Info("migrating db to latest version")
		err = migrator.Up()
	}
	if err != nil {
		switch err {
		case migrate.ErrNoChange:
			m.log.Warn("skip migrating db: no change detected")
		default:
			return errors.Wrap(err)
		}
	}
	m.Info(os.Stdout, true)
	return nil
}
