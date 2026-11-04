package planner

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/task"
)

const (
	timeFormat       = "2006-01-02 15:04:05.00"
	defaultTailBytes = 4096
)

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	es common.MessageSender

	tm task.Manager

	logDir    string
	tailBytes int

	sync.RWMutex
	todos map[string]*entity.Plan
}

func New(es common.MessageSender, opts ...Option) Manager {
	return newManager(es, opts...)
}

func newManager(es common.MessageSender, opts ...Option) *manager {
	m := &manager{
		es:    es,
		todos: make(map[string]*entity.Plan),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	m.tm = task.New(
		task.MaxConcurrency(10),
		task.WithLogger(m.log),
	)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	if m.es == nil {
		return nil
	}
	return []common.Service{m.es}
}
