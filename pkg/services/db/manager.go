package db

import (
	"database/sql"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

type manager struct {
	name string
	log  log.Logger

	dbtype     string
	source     Source
	migration  migrationConfig
	connection connectionConfig

	dialector gorm.Dialector
	ormDB     *gorm.DB
	sqlDB     *sql.DB
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(opts ...Option) *manager {
	m := &manager{
		log: log.Default,
	}
	m.apply(opts...)
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
