package pubsub

import (
	"context"
	"path"
	"sync"
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

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	published atomic.Uint64
}

func New(b driver.Driver, opts ...Option) Manager {
	return newManager(b, opts...)
}

func newManager(b driver.Driver, opts ...Option) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &manager{
		log:    log.Default,
		wg:     &sync.WaitGroup{},
		bus:    b,
		ctx:    ctx,
		cancel: cancel,
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
