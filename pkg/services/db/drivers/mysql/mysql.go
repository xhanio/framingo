// Package mysql registers the MySQL GORM dialector, golang-migrate driver,
// DSN builder, and cleanup hooks with pkg/services/db. Blank-import it to
// enable MySQL support.
package mysql

import (
	"database/sql"
	"fmt"

	migratedb "github.com/golang-migrate/migrate/v4/database"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/xhanio/errors"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/services/db"
)

func init() {
	db.Register(db.MySQL, db.Driver{
		Dialector: dialector,
		Migration: migration,
		DSN:       dsn,
		Cleanup:   cleanup,
	})
}

func dialector(dsn string) gorm.Dialector {
	return gormmysql.Open(dsn)
}

func migration(sqlDB *sql.DB) (migratedb.Driver, error) {
	return migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
}

func dsn(s db.Source) (string, error) {
	params := s.GetParams()
	if s.Secure {
		if _, ok := params["tls"]; !ok {
			params["tls"] = "true"
		}
	}
	value := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", s.User, s.Password, s.Host, s.Port, s.DBName)
	return db.AppendParams(value, params, "&", "&"), nil
}

func cleanup(gdb *gorm.DB, dbName string, schema bool) error {
	if schema {
		var name string
		if err := gdb.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := gdb.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := gdb.Exec(fmt.Sprintf("CREATE DATABASE %s", name)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := gdb.Exec(fmt.Sprintf("USE %s", name)).Error; err != nil {
			return errors.Wrap(err)
		}
		return nil
	}
	err := gdb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		defer tx.Exec("SET FOREIGN_KEY_CHECKS = 1")

		tables := []string{}
		if err := tx.Raw("SHOW TABLES").Pluck("Tables_in_"+dbName, &tables).Error; err != nil {
			return err
		}
		for _, table := range tables {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`", table)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err)
}
