package db

import (
	"database/sql"
	"fmt"
	"io"
	"path"

	"go.uber.org/zap/zapcore"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger

	dbtype     string
	source     Source
	migration  MigrationConfig
	connection ConnectionConfig

	dialector gorm.Dialector
	ormDB     *gorm.DB
	sqlDB     *sql.DB
}

func New(opts ...Option) Manager {
	m := &manager{}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) use(dbtype string, dsn string) (gorm.Dialector, error) {
	switch dbtype {
	case SQLite:
		return sqlite.Open(dsn), nil
	case MySQL:
		return mysql.Open(dsn), nil
	case Postgres:
		return postgres.Open(dsn), nil
	case Clickhouse:
		return clickhouse.Open(dsn), nil
	default:
		return nil, errors.Newf("unsupported db type:%s", dbtype)
	}
}

func (m *manager) connect(dbtype string, s Source) error {
	dsn, err := s.DSN(dbtype)
	if err != nil {
		return errors.Wrap(err)
	}
	dialector, err := m.use(dbtype, dsn)
	if err != nil {
		return errors.Wrap(err)
	}
	log := zapgorm2.New(m.log.Sugared().Desugar())
	if m.log.Level() == zapcore.DebugLevel {
		log.LogMode(logger.Info)
	} else {
		log.LogMode(logger.Silent)
	}
	log.IgnoreRecordNotFoundError = true
	gc := &gorm.Config{
		Logger: log,
	}
	ormDB, err := gorm.Open(dialector, gc)
	if err != nil {
		return errors.Wrap(err)
	}
	sqlDB, err := ormDB.DB()
	if err != nil {
		return errors.Wrap(err)
	}
	m.dialector = dialector
	m.ormDB = ormDB
	m.sqlDB = sqlDB
	return nil
}

func (m *manager) ORM() *gorm.DB {
	return m.ormDB
}

func (m *manager) DB() *sql.DB {
	return m.sqlDB
}

func (m *manager) Init() error {
	// connect to database
	err := m.connect(m.dbtype, m.source)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header(m.Name())
	if debug {
		t.Object(m.source)
		t.NewLine()
		t.Object(m.connection)
		t.NewLine()
		t.Object(m.migration)
		t.NewLine()
	}
	stats := m.sqlDB.Stats()
	t.Object(stats)
	t.NewLine()
	t.Flush()
}

func (m *manager) Cleanup(schema bool) error {
	db := m.ormDB

	switch m.dbtype {
	case Postgres:
		return m.cleanupPostgres(db, schema)
	case MySQL:
		return m.cleanupMySQL(db, schema)
	case SQLite:
		return m.cleanupSQLite(db, schema)
	case Clickhouse:
		return m.cleanupClickhouse(db, schema)
	default:
		return errors.Newf("cleanup operation not supported for database type: %s", m.dbtype)
	}
}

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
