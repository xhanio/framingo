package messagebus

import (
	"context"
	"path"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// DefaultTopic is the topic messagebus uses when WithTopic is not provided.
const DefaultTopic = "/messages"

var _ Manager = (*manager)(nil)

type manager struct {
	name  string
	log   log.Logger
	bus   model.Pubsub
	topic string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	modules map[string]common.Named
	started bool
}

func New(bus model.Pubsub, opts ...Option) Manager {
	return newManager(bus, opts...)
}

func newManager(bus model.Pubsub, opts ...Option) *manager {
	m := &manager{
		log:     log.Default,
		bus:     bus,
		topic:   DefaultTopic,
		modules: make(map[string]common.Named),
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
	return []common.Service{m.bus}
}
