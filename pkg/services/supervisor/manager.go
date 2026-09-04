package supervisor

import (
	"context"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type manager struct {
	name string
	log  log.Logger

	cancel context.CancelFunc
	wg     *sync.WaitGroup

	c       *controller
	monitor *monitor
}

func New(config confutil.Config, opts ...Option) Manager {
	return newManager(config, opts...)
}

func newManager(config confutil.Config, opts ...Option) *manager {
	c := newController(config)
	m := &manager{
		wg: &sync.WaitGroup{},
		c:  c,
		monitor: &monitor{
			c: c,
		},
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	m.c.log = m.log
	m.monitor.log = m.log
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
