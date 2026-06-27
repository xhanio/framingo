package db

import (
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/xhanio/errors"
)

func (m *manager) migrate(url string, version uint) error {
	d, err := lookupDriver(m.dbtype)
	if err != nil {
		return errors.Wrap(err)
	}
	if d.Migration == nil {
		return errors.Newf("driver %s does not provide a migration driver", m.dbtype)
	}
	driver, err := d.Migration(m.sqlDB)
	if err != nil {
		return errors.Wrap(err)
	}
	migrator, err := migrate.NewWithDatabaseInstance(url, m.source.DBName, driver)
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
