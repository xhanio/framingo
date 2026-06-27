// Package postgres registers the Postgres GORM dialector, golang-migrate
// driver, DSN builder, and cleanup hooks with pkg/services/db. Blank-import it
// to enable Postgres support.
package postgres

import (
	"database/sql"
	"fmt"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/xhanio/errors"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/services/db"
)

func init() {
	db.Register(db.Postgres, db.Driver{
		Dialector: dialector,
		Migration: migration,
		DSN:       dsn,
		Cleanup:   cleanup,
	})
}

func dialector(dsn string) gorm.Dialector {
	return gormpg.Open(dsn)
}

func migration(sqlDB *sql.DB) (migratedb.Driver, error) {
	return migratepg.WithInstance(sqlDB, &migratepg.Config{})
}

func dsn(s db.Source) (string, error) {
	params := s.GetParams()
	if _, ok := params["sslmode"]; !ok {
		if s.Secure {
			params["sslmode"] = "require"
		} else {
			params["sslmode"] = "disable"
		}
	}
	value := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", s.Host, s.Port, s.User, s.Password, s.DBName)
	return db.AppendParams(value, params, " ", " "), nil
}

func cleanup(gdb *gorm.DB, _ string, schema bool) error {
	if schema {
		if err := gdb.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
			return errors.Wrap(err)
		}
		if err := gdb.Exec("CREATE SCHEMA public").Error; err != nil {
			return errors.Wrap(err)
		}
		return nil
	}
	err := gdb.Transaction(func(tx *gorm.DB) error {
		tables := []string{}
		if err := tx.Raw("SELECT tablename FROM pg_tables WHERE schemaname='public'").Pluck("tablename", &tables).Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;", table)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err)
}
