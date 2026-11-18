package db

import (
	"maps"

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

// CloneParams returns a mutable copy, never nil.
func (s *Source) CloneParams() map[string]string {
	params := make(map[string]string)
	maps.Copy(params, s.Params)
	return params
}

// DSN fails unless the dbtype's driver package is blank-imported.
func (s *Source) DSN(dbtype string) (string, error) {
	d, err := lookupDriver(dbtype)
	if err != nil {
		return "", errors.Wrap(err)
	}
	if d.DSN == nil {
		return "", errors.Newf("driver %s does not provide a DSN builder", dbtype)
	}
	return d.DSN(*s)
}
