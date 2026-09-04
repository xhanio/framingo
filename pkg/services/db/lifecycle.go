package db

import (
	"context"
	"fmt"
	"io"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	// dynamic configs, read under this instance's root - db.connection.* for
	// the default instance, <name>.connection.* for a named one
	config := confutil.FromContext(ctx)
	root := nameutil.ConfigRoot(m.name)
	m.applyOptions(
		WithConnection(
			config.GetInt("%s.connection.max_open", root),
			config.GetInt("%s.connection.max_idle", root),
			config.GetDuration("%s.connection.max_lifetime", root),
			config.GetDuration("%s.connection.max_idle_time", root),
			config.GetDuration("%s.connection.exec_timeout", root),
		),
	)
	m.setDefaults()
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
