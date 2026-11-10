package messagebus

import (
	"context"
	"sync"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

// DefaultTopic is the topic messagebus uses when WithTopic is not provided.
const DefaultTopic = "/messages"

var _ Manager = (*manager)(nil)

type manager struct {
	name  string
	log   log.Logger
	bus   model.Pubsub
	topic string

	pingInterval time.Duration
	pingTimeout  time.Duration

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
		bus:          bus,
		pingInterval: DefaultPingInterval,
		pingTimeout:  DefaultPingTimeout,
		modules:      make(map[string]common.Named),
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
