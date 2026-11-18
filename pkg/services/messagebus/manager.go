package messagebus

import (
	"context"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

const DefaultTopic = "/messages"

var _ Manager = (*manager)(nil)

type manager struct {
	name  string
	log   log.Logger
	bus   model.Pubsub
	topic string

	ctx    context.Context // the latest Start's, for modules registered while it runs
	cancel context.CancelFunc
	wg     syncutil.WaitGroup // listeners, whichever Start they came from

	mu      sync.Mutex
	modules map[string]common.Named
	started bool
}

func New(bus model.Pubsub, opts ...Option) Manager {
	return newManager(bus, opts...)
}

func newManager(bus model.Pubsub, opts ...Option) *manager {
	m := &manager{
		bus:     bus,
		modules: make(map[string]common.Named),
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
	return []common.Service{m.bus}
}
