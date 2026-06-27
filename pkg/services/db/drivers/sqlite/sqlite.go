// Package sqlite registers the SQLite GORM dialector, golang-migrate driver,
// DSN builder, and cleanup hooks with pkg/services/db. Blank-import it to
// enable SQLite support without forcing every binary to pull in SQLite when
// they only use other engines.
package sqlite

import (
	"database/sql"
	"fmt"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/xhanio/errors"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/services/db"
)

func init() {
	db.Register(db.SQLite, db.Driver{
		Dialector: dialector,
		Migration: migration,
		DSN:       dsn,
		Cleanup:   cleanup,
	})
}

func dialector(dsn string) gorm.Dialector {
	return gormsqlite.Open(dsn)
}

func migration(sqlDB *sql.DB) (migratedb.Driver, error) {
	return migratesqlite.WithInstance(sqlDB, &migratesqlite.Config{})
}

func dsn(s db.Source) (string, error) {
	value := s.DBName
	if value == "" {
		value = ":memory:"
	}
	return db.AppendParams(value, s.GetParams(), "?", "&"), nil
}

func cleanup(gdb *gorm.DB, _ string, schema bool) error {
	if schema {
		err := gdb.Transaction(func(tx *gorm.DB) error {
			tables := []string{}
			if err := tx.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Pluck("name", &tables).Error; err != nil {
				return err
			}
			for _, table := range tables {
				if err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
					return err
				}
			}
			return nil
		})
		return errors.Wrap(err)
	}
	err := gdb.Transaction(func(tx *gorm.DB) error {
		tables := []string{}
		if err := tx.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Pluck("name", &tables).Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err)
}
