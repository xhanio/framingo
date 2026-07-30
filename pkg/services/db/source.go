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

// GetParams returns a defensive copy of s.Params so drivers can mutate it
// while building a DSN without affecting the caller's Source.
func (s *Source) GetParams() map[string]string {
	params := make(map[string]string)
	maps.Copy(params, s.Params)
	return params
}

// DSN resolves the driver registered for dbtype and asks it to format the DSN.
// The driver subpackage must be blank-imported for this to succeed.
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
