package db

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
)

type Type string

const (
	SQLite     = Type("sqlite")
	MySQL      = Type("mysql")
	Postgres   = Type("postgres")
	Clickhouse = Type("clickhouse")
)

type Source struct {
	Host     string
	Port     uint
	User     string
	Password string `print:"-"`
	DBName   string
}

func (s *Source) DSN(dbtype Type) (string, error) {
	switch dbtype {
	case MySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", s.User, s.Password, s.Host, s.Port, s.DBName), nil
	case Postgres:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", s.Host, s.Port, s.User, s.Password, s.DBName), nil
	case SQLite:
		return ":memory:", nil
	case Clickhouse:
		return fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s?", s.User, s.Password, s.Host, s.Port, s.DBName), nil
	default:
		return "", errors.Newf("unsupported db type %s", dbtype)
	}
}

type ConnectionConfig struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	ExecTimeout time.Duration
}

type MigrationConfig struct {
	Directory string
	Version   uint
}

type Manager interface {
	common.Service
	common.Initializable
	common.Debuggable
	ORM() *gorm.DB
	DB() *sql.DB
}
