package driver

import (
	"context"
	"strings"
	"sync"

	"github.com/xhanio/framingo/pkg/structs/trie"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type memoryDriver struct {
	*dispatcher
	mu sync.RWMutex

	topics *trie.Trie[[]*subscriber]
}

func NewMemory(log log.Logger, opts ...Option) Driver {
	return &memoryDriver{
		dispatcher: newDispatcher(log, opts...),
		topics:     trie.New[[]*subscriber](),
	}
}

func (b *memoryDriver) Subscribe(name string, topic string) (<-chan entity.PubsubMessage, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sub := newSubscriber(name, b.opts)

	if node, ok := b.topics.Find(topic); ok {
		subscribers := append(node.Value(), sub)
		b.topics.Remove(topic)
		b.topics.Add(topic, subscribers)
	} else {
		b.topics.Add(topic, []*subscriber{sub})
	}

	return sub.ch, nil
}

// GetSubscribers returns the names of all subscribers for the given topic and its parent topics.
func (b *memoryDriver) GetSubscribers(topic string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.getSubscribers(topic)
	names := make([]string, len(subs))
	for i, sub := range subs {
		names[i] = sub.name
	}
	return names
}

// getSubscribers returns all subscribers for the given topic and its parent topics.
func (b *memoryDriver) getSubscribers(topic string) []*subscriber {
	var subscribers []*subscriber

	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		if node, ok := b.topics.Find(prefix); ok {
			subscribers = append(subscribers, node.Value()...)
		}
	}

	return subscribers
}

func (b *memoryDriver) Unsubscribe(name string, topic string) error {
	if name == "" {
		return nil
	}

	b.mu.Lock()
	var removed []*subscriber
	if node, ok := b.topics.Find(topic); ok {
		subscribers := node.Value()
		filtered := make([]*subscriber, 0, len(subscribers))
		for _, sub := range subscribers {
			if sub.name == name {
				removed = append(removed, sub)
			} else {
				filtered = append(filtered, sub)
			}
		}

		b.topics.Remove(topic)
		if len(filtered) > 0 {
			b.topics.Add(topic, filtered)
		}
	}
	b.mu.Unlock()

	for _, sub := range removed {
		sub.stop()
	}
	return nil
}

func (b *memoryDriver) Publish(_ context.Context, from string, topic string, kind string, payload any) error {
	msg := entity.PubsubMessage{From: from, Topic: topic, Kind: kind, Payload: payload}

	var lagged []laggard

	b.mu.RLock()
	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		node, ok := b.topics.Find(prefix)
		if !ok {
			continue
		}
		for _, sub := range node.Value() {
			if from != "" && sub.name == from {
				continue
			}
			if b.offer(sub, prefix, msg) {
				lagged = append(lagged, laggard{sub: sub, topic: prefix})
			}
		}
	}
	b.mu.RUnlock()

	// Eviction needs the write lock, which cannot be taken while Publish holds
	// the read lock: Go's RWMutex is not upgradable.
	b.evict(lagged)
	return nil
}

func (b *memoryDriver) evict(lagged []laggard) {
	for _, l := range lagged {
		if !b.claim(l) {
			continue
		}
		b.mu.Lock()
		b.remove(l.topic, l.sub)
		b.mu.Unlock()

		l.sub.stop()
	}
}

// remove drops target from topic by identity. Removing by name would race a
// subscriber that unsubscribed and resubscribed under the same name between
// the read lock being released and the write lock being taken.
func (b *memoryDriver) remove(topic string, target *subscriber) {
	node, ok := b.topics.Find(topic)
	if !ok {
		return
	}
	subscribers := node.Value()
	filtered := make([]*subscriber, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub != target {
			filtered = append(filtered, sub)
		}
	}

	b.topics.Remove(topic)
	if len(filtered) > 0 {
		b.topics.Add(topic, filtered)
	}
}

func (b *memoryDriver) Start(ctx context.Context) error {
	return nil
}

func (b *memoryDriver) Stop(wait bool) error {
	b.mu.Lock()
	var stopped []*subscriber
	for _, key := range b.topics.Keys() {
		if node, ok := b.topics.Find(key); ok {
			stopped = append(stopped, node.Value()...)
		}
		b.topics.Remove(key)
	}
	b.mu.Unlock()

	for _, sub := range stopped {
		sub.stop()
	}
	if wait {
		for _, sub := range stopped {
			sub.wait()
		}
	}
	return nil
}
