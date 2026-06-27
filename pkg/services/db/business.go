package db

import (
	"database/sql"
	"fmt"

	"github.com/xhanio/errors"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

func (m *manager) use(dbtype string, dsn string) (gorm.Dialector, error) {
	d, err := lookupDriver(dbtype)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if d.Dialector == nil {
		return nil, errors.Newf("driver %s does not provide a GORM dialector", dbtype)
	}
	return d.Dialector(dsn), nil
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
	d, err := lookupDriver(m.dbtype)
	if err != nil {
		return errors.Wrap(err)
	}
	if d.Cleanup == nil {
		return errors.Newf("cleanup operation not supported for database type: %s", m.dbtype)
	}
	return d.Cleanup(m.ormDB, m.source.DBName, schema)
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
