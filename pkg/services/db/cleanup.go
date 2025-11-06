package db

import (
	"fmt"

	"github.com/xhanio/errors"
	"gorm.io/gorm"
)

func (m *manager) cleanupPostgres(db *gorm.DB, schema bool) error {
	if schema {
		if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
			return errors.Wrap(err)
		}
		if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
			return errors.Wrap(err)
		}
	} else {
		err := db.Transaction(func(tx *gorm.DB) error {
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
	return nil
}

func (m *manager) cleanupMySQL(db *gorm.DB, schema bool) error {
	if schema {
		// Get database name
		var dbName string
		if err := db.Raw("SELECT DATABASE()").Scan(&dbName).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := db.Exec(fmt.Sprintf("USE %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
	} else {
		err := db.Transaction(func(tx *gorm.DB) error {
			// Disable foreign key checks
			if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
				return err
			}
			defer tx.Exec("SET FOREIGN_KEY_CHECKS = 1")

			tables := []string{}
			if err := tx.Raw("SHOW TABLES").Pluck("Tables_in_"+m.source.DBName, &tables).Error; err != nil {
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
	return nil
}

func (m *manager) cleanupSQLite(db *gorm.DB, schema bool) error {
	if schema {
		// For SQLite, drop all tables and recreate the schema
		err := db.Transaction(func(tx *gorm.DB) error {
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
	} else {
		err := db.Transaction(func(tx *gorm.DB) error {
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
}

func (m *manager) cleanupClickhouse(db *gorm.DB, schema bool) error {
	if schema {
		// Get database name
		dbName := m.source.DBName
		if err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
		if err := db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error; err != nil {
			return errors.Wrap(err)
		}
	} else {
		err := db.Transaction(func(tx *gorm.DB) error {
			tables := []string{}
			if err := tx.Raw(fmt.Sprintf("SHOW TABLES FROM %s", m.source.DBName)).Pluck("name", &tables).Error; err != nil {
				return err
			}
			for _, table := range tables {
				if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s.%s", m.source.DBName, table)).Error; err != nil {
					return err
				}
			}
			return nil
		})
		return errors.Wrap(err)
	}
	return nil
}
