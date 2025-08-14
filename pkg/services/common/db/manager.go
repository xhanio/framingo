package db

import (
	"database/sql"
	"io"
	"path"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"go.uber.org/zap/zapcore"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

type manager struct {
	name string
	log  log.Logger

	dbtype     Type
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

func (m *manager) use(dbtype Type, dsn string) (gorm.Dialector, error) {
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

func (m *manager) connect(dbtype Type, s Source) error {
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
