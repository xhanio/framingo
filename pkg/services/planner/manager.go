package planner

import (
	"path"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/task"
)

const timeFormat = "2006-01-02 15:04:05.00"

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	es common.MessageSender

	tm task.Manager

	sync.RWMutex
	todos map[string]*entity.Plan
}

func New(es common.MessageSender, opts ...Option) Manager {
	return newManager(es, opts...)
}

func newManager(es common.MessageSender, opts ...Option) *manager {
	m := &manager{
		log:   log.Default,
		es:    es,
		todos: make(map[string]*entity.Plan),
	}
	m.apply(opts...)
	m.log = m.log.By(m)
	m.tm = task.New(
		task.MaxConcurrency(10),
		task.WithLogger(m.log),
	)
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
