package db

import (
	"context"
	"fmt"
	"io"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	// dynamic configs
	config := confutil.FromContext(ctx)
	m.apply(
		WithConnection(
			config.GetInt("db.connection.max_open"),
			config.GetInt("db.connection.max_idle"),
			config.GetDuration("db.connection.max_lifetime"),
			config.GetDuration("db.connection.max_idle_time"),
			config.GetDuration("db.connection.exec_timeout"),
		),
	)
	// connect to database
	err := m.connect(m.dbtype, m.source)
	if err != nil {
		return errors.Wrap(err)
	}
	// set up connection pool
	m.sqlDB.SetMaxOpenConns(m.connection.MaxOpen)
	m.sqlDB.SetMaxIdleConns(m.connection.MaxIdle)
	m.sqlDB.SetConnMaxLifetime(m.connection.MaxLifetime)
	m.sqlDB.SetConnMaxIdleTime(m.connection.MaxIdleTime)
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
