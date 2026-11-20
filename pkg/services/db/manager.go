package db

import (
	"database/sql"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type manager struct {
	name string
	log  log.Logger

	dbtype     string
	source     Source
	migration  migrationConfig
	connection connectionConfig

	plugins []gorm.Plugin

	// what queries read: every Init builds a new one and swaps it in whole
	pool atomic.Pointer[pool]
}

// pool is one connect's result, never changed once stored: a restart replaces
// it while the queries already holding the previous one run on.
type pool struct {
	dialector   gorm.Dialector
	orm         *gorm.DB
	sql         *sql.DB
	execTimeout time.Duration
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
