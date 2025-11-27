package db

import (
	"fmt"
	"maps"
	"sort"

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
	Secure   bool
	Params   map[string]string
}

func (s *Source) getParams() map[string]string {
	params := make(map[string]string)
	maps.Copy(params, s.Params)
	return params
}

func (s *Source) DSN(dbtype string) (string, error) {
	params := s.getParams()

	switch dbtype {
	case MySQL:
		if s.Secure {
			if _, ok := params["tls"]; !ok {
				params["tls"] = "true"
			}
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local", s.User, s.Password, s.Host, s.Port, s.DBName)
		return s.appendParams(dsn, params, "&", "&"), nil
	case Postgres:
		if _, ok := params["sslmode"]; !ok {
			if s.Secure {
				params["sslmode"] = "require"
			} else {
				params["sslmode"] = "disable"
			}
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", s.Host, s.Port, s.User, s.Password, s.DBName)
		return s.appendParams(dsn, params, " ", " "), nil
	case SQLite:
		dsn := s.DBName
		if dsn == "" {
			dsn = ":memory:"
		}
		return s.appendParams(dsn, params, "?", "&"), nil
	case Clickhouse:
		if s.Secure {
			if _, ok := params["secure"]; !ok {
				params["secure"] = "true"
			}
		}
		dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s?", s.User, s.Password, s.Host, s.Port, s.DBName)
		return s.appendParams(dsn, params, "&", "&"), nil
	default:
		return "", errors.Newf("unsupported db type %s", dbtype)
	}
}

func (s *Source) appendParams(dsn string, params map[string]string, firstSep, sep string) string {
	if len(params) == 0 {
		return dsn
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i == 0 {
			dsn += firstSep
		} else {
			dsn += sep
		}
		dsn += fmt.Sprintf("%s=%s", k, params[k])
	}
	return dsn
}
