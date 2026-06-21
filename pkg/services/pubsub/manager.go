package pubsub

import (
	"path"
	"sync/atomic"

	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	bus driver.Driver

	published atomic.Uint64
}

func New(b driver.Driver, opts ...Option) Manager {
	return newManager(b, opts...)
}

func newManager(b driver.Driver, opts ...Option) *manager {
	m := &manager{
		log: log.Default,
		bus: b,
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
