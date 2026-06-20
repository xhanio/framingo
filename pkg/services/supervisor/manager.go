package supervisor

import (
	"context"
	"path"
	"sync"

	"github.com/spf13/viper"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger

	cancel context.CancelFunc
	wg     *sync.WaitGroup

	c       *controller
	monitor *monitor
}

func New(config *viper.Viper, opts ...Option) Manager {
	return newManager(config, opts...)
}

func newManager(config *viper.Viper, opts ...Option) *manager {
	c := newController(config)
	m := &manager{
		log: log.Default,
		wg:  &sync.WaitGroup{},
		c:   c,
		monitor: &monitor{
			c: c,
		},
	}
	m.apply(opts...)
	m.log = m.log.By(m)
	m.c.log = m.log
	m.monitor.log = m.log
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
