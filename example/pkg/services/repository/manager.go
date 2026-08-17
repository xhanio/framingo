package repository

import (
	"context"
	"database/sql"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type manager struct {
	name string
	log  log.Logger
	db   model.Database
}

func New(db model.Database, opts ...Option) Repository {
	return newManager(db, opts...)
}

func newManager(db model.Database, opts ...Option) *manager {
	m := &manager{db: db}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return []common.Service{m.db}
}

func (m *manager) Transaction(ctx context.Context, fn func(context.Context) error, opts ...*sql.TxOptions) error {
	return m.db.Transaction(ctx, fn, opts...)
}
