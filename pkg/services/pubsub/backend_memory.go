package pubsub

import (
	"strings"
	"sync"

	"github.com/xhanio/framingo/pkg/structs/trie"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// memoryBackend implements an in-memory pub/sub system using a trie data structure
// for efficient hierarchical topic matching.
type memoryBackend struct {
	log log.Logger
	sync.RWMutex
	topics *trie.Trie[[]common.Named]
}

// NewMemoryBackend creates a new in-memory backend for pub/sub.
func NewMemoryBackend(log log.Logger) Backend {
	return &memoryBackend{
		log:    log,
		topics: trie.New[[]common.Named](),
	}
}

// Subscribe registers a service to receive events for the given topic.
func (b *memoryBackend) Subscribe(svc common.Named, topic string) error {
	if svc == nil {
		return nil
	}

	b.Lock()
	defer b.Unlock()

	if node, ok := b.topics.Find(topic); ok {
		subscribers := append(node.Value(), svc)
		b.topics.Remove(topic)
		b.topics.Add(topic, subscribers)
	} else {
		b.topics.Add(topic, []common.Named{svc})
	}

	return nil
}

// GetSubscribers returns all subscribers for the given topic and its parent topics.
// For hierarchical matching: publishing to "app/module/component" returns subscribers of
// "app", "app/module", and "app/module/component".
func (b *memoryBackend) GetSubscribers(topic string) []common.Named {
	b.RLock()
	defer b.RUnlock()

	var subscribers []common.Named

	// Find all subscribers for this topic and its parent topics
	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		if node, ok := b.topics.Find(prefix); ok {
			subscribers = append(subscribers, node.Value()...)
		}
	}

	return subscribers
}

// Unsubscribe removes a service from receiving events for the given topic.
func (b *memoryBackend) Unsubscribe(svc common.Named, topic string) error {
	if svc == nil {
		return nil
	}

	b.Lock()
	defer b.Unlock()

	if node, ok := b.topics.Find(topic); ok {
		subscribers := node.Value()
		filtered := make([]common.Named, 0, len(subscribers))
		for _, sub := range subscribers {
			if sub != svc {
				filtered = append(filtered, sub)
			}
		}

		b.topics.Remove(topic)
		if len(filtered) > 0 {
			b.topics.Add(topic, filtered)
		}
	}

	return nil
}

// Close cleans up resources. For in-memory backend, this is a no-op.
func (b *memoryBackend) Close() error {
	return nil
}

// SetDispatcher sets the dispatch function. For in-memory backend, this is a no-op
// since it doesn't need to dispatch events (the manager handles all dispatching).
func (b *memoryBackend) SetDispatcher(dispatch DispatchFunc) {
	// No-op for memory backend
}
