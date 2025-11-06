package db

import (
	"fmt"

	"github.com/xhanio/errors"
)

const (
	SQLite     = "sqlite"
	MySQL      = "mysql"
	Postgres   = "postgres"
	Clickhouse = "clickhouse"
)

type Source struct {
	Host     string
	Port     uint
	User     string
	Password string `print:"-"`
	DBName   string
}

func (s *Source) DSN(dbtype string) (string, error) {
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
