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
	m.connection = m.resolve(confutil.FromContext(ctx))
	err := m.connect(m.dbtype, m.source)
	if err != nil {
		return errors.Wrap(err)
	}
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
	if sqlDB := m.DB(); sqlDB != nil {
		t.Object(sqlDB.Stats())
	}
	t.NewLine()
	t.Flush()
}
