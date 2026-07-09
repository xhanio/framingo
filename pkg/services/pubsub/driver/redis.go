package driver

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type redisDriver struct {
	*dispatcher

	ctx    context.Context
	cancel context.CancelFunc

	client *redis.Client
	pubsub *redis.PubSub

	mu       sync.RWMutex
	topics   map[string][]*subscriber // topic -> local subscribers
	patterns map[string]bool          // track subscribed Redis patterns

	wg sync.WaitGroup
}

func NewRedis(client *redis.Client, log log.Logger, opts ...Option) (Driver, error) {
	if client == nil {
		return nil, errors.Newf("redis client cannot be nil")
	}

	return &redisDriver{
		dispatcher: newDispatcher(log, opts...),
		client:     client,
		topics:     make(map[string][]*subscriber),
		patterns:   make(map[string]bool),
	}, nil
}

func (b *redisDriver) Subscribe(name string, topic string) (<-chan entity.PubsubMessage, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sub := newSubscriber(name, b.opts)
	b.topics[topic] = append(b.topics[topic], sub)

	pattern := b.getTopicPattern(topic)
	if !b.patterns[pattern] {
		b.patterns[pattern] = true
		if b.pubsub != nil {
			if err := b.pubsub.PSubscribe(b.ctx, pattern); err != nil {
				return nil, errors.Wrapf(err, "failed to subscribe to redis pattern")
			}
		}
	}

	return sub.ch, nil
}

// GetSubscribers returns the names of local subscribers matching the given topic hierarchically.
func (b *redisDriver) GetSubscribers(topic string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.getSubscribers(topic)
	names := make([]string, len(subs))
	for i, sub := range subs {
		names[i] = sub.name
	}
	return names
}

// getSubscribers returns local subscribers matching the given topic hierarchically.
func (b *redisDriver) getSubscribers(topic string) []*subscriber {
	var subscribers []*subscriber
	for subTopic, subs := range b.topics {
		if topicMatches(subTopic, topic) {
			subscribers = append(subscribers, subs...)
		}
	}
	return subscribers
}

func (b *redisDriver) Unsubscribe(name string, topic string) error {
	if name == "" {
		return nil
	}

	b.mu.Lock()
	subscribers, ok := b.topics[topic]
	if !ok {
		b.mu.Unlock()
		return nil
	}

	var removed []*subscriber
	filtered := make([]*subscriber, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub.name == name {
			removed = append(removed, sub)
		} else {
			filtered = append(filtered, sub)
		}
	}

	var err error
	if len(filtered) > 0 {
		b.topics[topic] = filtered
	} else {
		err = b.unsubscribePattern(topic)
	}
	b.mu.Unlock()

	// stop tears down the pump, which owns close(ch); doing it here rather than
	// under the lock keeps close off the critical section.
	for _, sub := range removed {
		sub.stop()
	}
	return err
}

// unsubscribePattern drops a topic with no remaining subscribers. Callers must
// hold the write lock.
func (b *redisDriver) unsubscribePattern(topic string) error {
	delete(b.topics, topic)
	pattern := b.getTopicPattern(topic)
	delete(b.patterns, pattern)
	if b.pubsub != nil {
		return b.pubsub.PUnsubscribe(b.ctx, pattern)
	}
	return nil
}

func (b *redisDriver) evict(lagged []laggard) {
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
func (b *redisDriver) remove(topic string, target *subscriber) {
	subscribers, ok := b.topics[topic]
	if !ok {
		return
	}
	filtered := make([]*subscriber, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub != target {
			filtered = append(filtered, sub)
		}
	}
	if len(filtered) > 0 {
		b.topics[topic] = filtered
		return
	}
	if err := b.unsubscribePattern(topic); err != nil {
		b.log.Errorf("failed to unsubscribe redis pattern for topic %s: %v", topic, err)
	}
}

func (b *redisDriver) Start(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pubsub == nil {
		b.pubsub = b.client.PSubscribe(b.ctx)
	}

	for pattern := range b.patterns {
		if err := b.pubsub.PSubscribe(b.ctx, pattern); err != nil {
			return errors.Wrapf(err, "failed to subscribe to redis pattern %s", pattern)
		}
	}

	b.wg.Add(1)
	go b.listenForMessages()
	return nil
}

func (b *redisDriver) Stop(wait bool) error {
	if b.cancel != nil {
		b.cancel()
	}

	b.mu.Lock()
	var stopped []*subscriber
	for topic, subs := range b.topics {
		stopped = append(stopped, subs...)
		delete(b.topics, topic)
	}
	b.mu.Unlock()

	for _, sub := range stopped {
		sub.stop()
	}

	if wait {
		b.wg.Wait()
		for _, sub := range stopped {
			sub.wait()
		}
	}

	if b.pubsub != nil {
		err := b.pubsub.Close()
		b.pubsub = nil
		return err
	}
	return nil
}

// Publish dispatches locally and sends to Redis for cross-instance delivery.
func (b *redisDriver) Publish(ctx context.Context, from string, topic string, kind string, payload any) error {
	msg := entity.PubsubMessage{From: from, Topic: topic, Kind: kind, Payload: payload}

	b.mu.RLock()
	lagged := b.fanout(b.topics, from, msg)
	b.mu.RUnlock()

	// Eviction needs the write lock, which cannot be taken while Publish holds
	// the read lock: Go's RWMutex is not upgradable.
	b.evict(lagged)

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal event payload")
	}

	em := eventMessage{
		Publisher: from,
		Topic:     topic,
		Kind:      kind,
		Payload:   rawPayload,
	}

	data, err := json.Marshal(em)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal event message")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	channel := b.getRedisChannel(topic)
	return b.client.Publish(ctx, channel, data).Err()
}

func (b *redisDriver) listenForMessages() {
	defer b.wg.Done()

	ch := b.pubsub.Channel()
	for {
		select {
		case <-b.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			b.handleRedisMessage(msg)
		}
	}
}

func (b *redisDriver) handleRedisMessage(msg *redis.Message) {
	var eventMsg eventMessage
	if err := json.Unmarshal([]byte(msg.Payload), &eventMsg); err != nil {
		b.log.Errorf("failed to unmarshal redis event: %v", err)
		return
	}

	m := entity.PubsubMessage{
		From:    eventMsg.Publisher,
		Topic:   eventMsg.Topic,
		Kind:    eventMsg.Kind,
		Payload: eventMsg.Payload,
	}

	b.mu.RLock()
	lagged := b.fanout(b.topics, eventMsg.Publisher, m)
	b.mu.RUnlock()

	b.evict(lagged)
}

func (b *redisDriver) getRedisChannel(topic string) string {
	return "pubsub:" + topic
}

func (b *redisDriver) getTopicPattern(topic string) string {
	return "pubsub:" + topic + "*"
}
