package pubsub

import (
	"path"
	"strings"
	"sync"

	"github.com/xhanio/framingo/pkg/structs/trie"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger

	sync.RWMutex
	topics *trie.Trie[[]common.Named]
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		topics: trie.New[[]common.Named](),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
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

func (m *manager) Publish(svc common.Named, topic string, e common.Event) {
	if e == nil {
		return
	}

	m.RLock()
	var subscribers []common.Named

	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		if node, ok := m.topics.Find(prefix); ok {
			subscribers = append(subscribers, node.Value()...)
		}
	}
	m.RUnlock()

	for _, subscriber := range subscribers {
		go func(sub common.Named) {
			if eh, ok := sub.(common.EventHandler); ok {
				if err := eh.HandleEvent(e); err != nil {
					m.log.Error("error handling event", "topic", topic, "subscriber", sub.Name(), "error", err)
				}
			} else if reh, ok := sub.(common.RawEventHandler); ok {
				if err := reh.HandleRawEvent(e.Kind(), e); err != nil {
					m.log.Error("error handling raw event", "topic", topic, "subscriber", sub.Name(), "error", err)
				}
			}
		}(subscriber)
	}
}

func (m *manager) Subscribe(svc common.Named, topic string) {
	if svc == nil {
		return
	}
	m.Lock()
	defer m.Unlock()
	if node, ok := m.topics.Find(topic); ok {
		subscribers := append(node.Value(), svc)
		m.topics.Remove(topic)
		m.topics.Add(topic, subscribers)
	} else {
		m.topics.Add(topic, []common.Named{svc})
	}
}
