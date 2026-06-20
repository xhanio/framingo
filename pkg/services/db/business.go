package db

import (
	"database/sql"
	"fmt"

	"github.com/xhanio/errors"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

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

func (m *manager) Reload() error {
	err := m.Cleanup(true)
	if err != nil {
		return errors.Wrap(err)
	}
	if m.migration.Directory != "" {
		err := m.migrate(fmt.Sprintf("file://%s", m.migration.Directory), m.migration.Version)
		if err != nil {
			return errors.Wrap(err)
		}
	}
	return nil
}
