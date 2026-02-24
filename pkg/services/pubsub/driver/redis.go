package driver

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type redisDriver struct {
	log log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	client *redis.Client
	pubsub *redis.PubSub

	mu       sync.RWMutex
	topics   map[string][]subscriber // topic -> local subscribers
	patterns map[string]bool         // track subscribed Redis patterns

	wg sync.WaitGroup
}

func NewRedis(client *redis.Client, log log.Logger) (Driver, error) {
	if client == nil {
		return nil, errors.Newf("redis client cannot be nil")
	}

	return &redisDriver{
		log:      log,
		client:   client,
		topics:   make(map[string][]subscriber),
		patterns: make(map[string]bool),
	}, nil
}

func (b *redisDriver) Subscribe(name string, topic string) (<-chan Message, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, channelBufferSize)
	sub := subscriber{name: name, ch: ch}
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

	return ch, nil
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
func (b *redisDriver) getSubscribers(topic string) []subscriber {
	var subscribers []subscriber
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
	defer b.mu.Unlock()

	subscribers, ok := b.topics[topic]
	if !ok {
		return nil
	}

	filtered := make([]subscriber, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub.name == name {
			close(sub.ch)
		} else {
			filtered = append(filtered, sub)
		}
	}

	if len(filtered) > 0 {
		b.topics[topic] = filtered
	} else {
		delete(b.topics, topic)
		pattern := b.getTopicPattern(topic)
		delete(b.patterns, pattern)
		if b.pubsub != nil {
			_ = b.pubsub.PUnsubscribe(b.ctx, pattern)
		}
	}

	return nil
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
	for topic, subs := range b.topics {
		for _, sub := range subs {
			close(sub.ch)
		}
		delete(b.topics, topic)
	}
	b.mu.Unlock()

	if wait {
		b.wg.Wait()
	}

	if b.pubsub != nil {
		err := b.pubsub.Close()
		b.pubsub = nil
		return err
	}
	return nil
}

// Publish dispatches locally and sends to Redis for cross-instance delivery.
func (b *redisDriver) Publish(from string, topic string, kind string, payload any) error {
	b.mu.RLock()
	msg := Message{From: from, Topic: topic, Kind: kind, Payload: payload}
	for subTopic, subs := range b.topics {
		if topicMatches(subTopic, topic) {
			for _, sub := range subs {
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
	b.mu.RUnlock()

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

	ctx := b.ctx
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

	b.mu.RLock()
	defer b.mu.RUnlock()

	m := Message{
		From:    eventMsg.Publisher,
		Topic:   eventMsg.Topic,
		Kind:    eventMsg.Kind,
		Payload: eventMsg.Payload,
	}

	for subTopic, subs := range b.topics {
		if topicMatches(subTopic, eventMsg.Topic) {
			for _, sub := range subs {
				if eventMsg.Publisher != "" && sub.name == eventMsg.Publisher {
					continue
				}
				select {
				case sub.ch <- m:
				default:
				}
			}
		}
	}
}

func (b *redisDriver) getRedisChannel(topic string) string {
	return "pubsub:" + topic
}

func (b *redisDriver) getTopicPattern(topic string) string {
	return "pubsub:" + topic + "*"
}
