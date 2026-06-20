package repository

import (
	"context"
	"database/sql"
	"path"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger
	db   model.Database
}

func New(db model.Database, opts ...Option) Repository {
	return newRepo(db, opts...)
}

func newRepo(db model.Database, opts ...Option) *manager {
	m := &manager{db: db}
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
	return []common.Service{m.db}
}

func (m *manager) Transaction(ctx context.Context, fn func(context.Context) error, opts ...*sql.TxOptions) error {
	return m.db.Transaction(ctx, fn, opts...)
}
