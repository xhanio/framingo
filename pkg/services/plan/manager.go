package plan

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
)

const (
	timeFormat       = "2006-01-02 15:04:05.00"
	defaultTailBytes = 4096
)

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	ms common.MessageSender

	tm model.Task

	logDir    string
	tailBytes int

	sync.RWMutex
	plans map[string]*registration
}

func New(ms common.MessageSender, tm model.Task, opts ...Option) Manager {
	return newManager(ms, tm, opts...)
}

func newManager(ms common.MessageSender, tm model.Task, opts ...Option) *manager {
	m := &manager{
		ms:    ms,
		tm:    tm,
		plans: make(map[string]*registration),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	deps := []common.Service{m.tm}
	if m.ms != nil {
		deps = append(deps, m.ms)
	}
	return deps
}
