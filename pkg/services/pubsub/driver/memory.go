package driver

import (
	"context"
	"strings"
	"sync"

	"github.com/xhanio/framingo/pkg/structs/trie"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type memoryDriver struct {
	log log.Logger
	mu  sync.RWMutex

	topics *trie.Trie[[]subscriber]
}

func NewMemory(log log.Logger) Driver {
	return &memoryDriver{
		log:    log,
		topics: trie.New[[]subscriber](),
	}
}

func (b *memoryDriver) Subscribe(name string, topic string) (<-chan Message, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, channelBufferSize)
	sub := subscriber{name: name, ch: ch}

	if node, ok := b.topics.Find(topic); ok {
		subscribers := append(node.Value(), sub)
		b.topics.Remove(topic)
		b.topics.Add(topic, subscribers)
	} else {
		b.topics.Add(topic, []subscriber{sub})
	}

	return ch, nil
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
func (b *memoryDriver) getSubscribers(topic string) []subscriber {
	var subscribers []subscriber

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
	defer b.mu.Unlock()

	if node, ok := b.topics.Find(topic); ok {
		subscribers := node.Value()
		filtered := make([]subscriber, 0, len(subscribers))
		for _, sub := range subscribers {
			if sub.name == name {
				close(sub.ch)
			} else {
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

func (b *memoryDriver) Publish(from string, topic string, kind string, payload any) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	msg := Message{From: from, Topic: topic, Kind: kind, Payload: payload}
	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		if node, ok := b.topics.Find(prefix); ok {
			for _, sub := range node.Value() {
				if from != "" && sub.name == from {
					continue
				}
				select {
				case sub.ch <- msg:
				default:
				}
			}
		}
	}
	return nil
}

func (b *memoryDriver) Start(ctx context.Context) error {
	return nil
}

func (b *memoryDriver) Stop(wait bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, key := range b.topics.Keys() {
		if node, ok := b.topics.Find(key); ok {
			for _, sub := range node.Value() {
				close(sub.ch)
			}
		}
		b.topics.Remove(key)
	}
	return nil
}
