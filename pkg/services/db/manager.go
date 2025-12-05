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
	m := &manager{
		log: log.Default,
	}
	for _, opt := range opts {
		opt(m)
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
	// set up connection pool
	m.sqlDB.SetMaxOpenConns(m.connection.MaxOpen)
	m.sqlDB.SetMaxIdleConns(m.connection.MaxIdle)
	m.sqlDB.SetConnMaxLifetime(m.connection.MaxLifetime)
	// migration
	if m.migration.Directory != "" {
		err = m.migrate(fmt.Sprintf("file://%s", m.migration.Directory), m.migration.Version)
		if err != nil {
			return errors.Wrap(err)
		}
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
