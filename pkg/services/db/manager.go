package db

import (
	"database/sql"
	"path"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
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
	m := &manager{
		log: log.Default,
	}
	m.apply(opts...)
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
