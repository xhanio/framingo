// Package clickhouse registers the ClickHouse GORM dialector, golang-migrate
// driver, DSN builder, and cleanup hooks with pkg/services/db. Blank-import it
// to enable ClickHouse support.
package clickhouse

import (
	"database/sql"
	"fmt"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	migratech "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/xhanio/errors"
	gormch "gorm.io/driver/clickhouse"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/services/db"
)

func init() {
	db.Register(db.Clickhouse, db.Driver{
		Dialector: dialector,
		Migration: migration,
		DSN:       dsn,
		Cleanup:   cleanup,
	})
}

func dialector(dsn string) gorm.Dialector {
	return gormch.Open(dsn)
}

func migration(sqlDB *sql.DB) (migratedb.Driver, error) {
	return migratech.WithInstance(sqlDB, &migratech.Config{})
}

func dsn(s db.Source) (string, error) {
	params := s.GetParams()
	if s.Secure {
		if _, ok := params["secure"]; !ok {
			params["secure"] = "true"
		}
	}
	value := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s?", s.User, s.Password, s.Host, s.Port, s.DBName)
	return db.AppendParams(value, params, "&", "&"), nil
}

func cleanup(gdb *gorm.DB, dbName string, schema bool) error {
	if schema {
		if err := gdb.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := gdb.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
		return nil
	}
	err := gdb.Transaction(func(tx *gorm.DB) error {
		tables := []string{}
		if err := tx.Raw(fmt.Sprintf("SHOW TABLES FROM %s", dbName)).Pluck("name", &tables).Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s.%s", dbName, table)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err)
}
