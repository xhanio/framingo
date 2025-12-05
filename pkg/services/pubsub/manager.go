package pubsub

import (
	"path"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name    string
	log     log.Logger
	backend Backend
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		log: log.Default,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.log = m.log.By(m)

	// Initialize backend if not provided
	if m.backend == nil {
		m.backend = NewMemoryBackend(m.log)
	}

	// Set the dispatcher for the backend so it can delegate event delivery to the manager
	m.backend.SetDispatcher(m.dispatch)

	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Publish(svc common.Named, topic string, e common.Event) {
	if e == nil {
		return
	}

	// Get all subscribers for this topic from the backend
	subscribers := m.backend.GetSubscribers(topic)

	// Dispatch to all local subscribers
	m.dispatch(subscribers, topic, e)

	// If using Redis backend, also publish to Redis for distribution to other instances
	if rb, ok := m.backend.(interface {
		PublishToRedis(svc common.Named, topic string, e common.Event) error
	}); ok {
		if err := rb.PublishToRedis(svc, topic, e); err != nil {
			m.log.Error("failed to publish to Redis", "topic", topic, "error", err)
		}
	}
}

// dispatch sends an event to all subscribers asynchronously.
// It handles both EventHandler and RawEventHandler interfaces independently.
func (m *manager) dispatch(subscribers []common.Named, topic string, e common.Event) {
	for _, subscriber := range subscribers {
		// Handle EventHandler interface
		if eh, ok := subscriber.(common.EventHandler); ok {
			go func(sub common.EventHandler, name string) {
				if err := sub.HandleEvent(e); err != nil {
					m.log.Error("error handling event", "topic", topic, "subscriber", name, "error", err)
				}
			}(eh, subscriber.Name())
		}

		// Handle RawEventHandler interface (independently of EventHandler)
		if reh, ok := subscriber.(common.RawEventHandler); ok {
			go func(sub common.RawEventHandler, name string) {
				if err := sub.HandleRawEvent(e.Kind(), e); err != nil {
					m.log.Error("error handling raw event", "topic", topic, "subscriber", name, "error", err)
				}
			}(reh, subscriber.Name())
		}
	}
}

func (m *manager) Subscribe(svc common.Named, topic string) {
	if err := m.backend.Subscribe(svc, topic); err != nil {
		m.log.Error("failed to subscribe", "topic", topic, "service", svc.Name(), "error", err)
	}
}
