package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// redisBackend implements a distributed pub/sub system using Redis.
// It supports hierarchical topic matching by subscribing to multiple Redis channels.
type redisBackend struct {
	log       log.Logger
	client    *redis.Client
	ctx       context.Context
	cancel    context.CancelFunc
	pubsub    *redis.PubSub
	mu        sync.RWMutex
	localSubs map[string][]common.Named // topic -> subscribers map
	dispatch  DispatchFunc               // dispatch function from manager
	started   bool
}

// eventMessage is the structure used to serialize events over Redis.
type eventMessage struct {
	Publisher string `json:"publisher"`
	Topic     string `json:"topic"`
	Kind      string `json:"kind"`
	Payload   any    `json:"payload"`
}

// NewRedisBackend creates a new Redis-based backend for pub/sub.
func NewRedisBackend(client *redis.Client, log log.Logger) (Backend, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &redisBackend{
		log:       log,
		client:    client,
		ctx:       ctx,
		cancel:    cancel,
		localSubs: make(map[string][]common.Named),
	}

	return b, nil
}

// Subscribe registers a service to receive events for the given topic.
// In Redis backend, this maintains a local registry and subscribes to Redis patterns.
func (b *redisBackend) Subscribe(svc common.Named, topic string) error {
	if svc == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Add to local subscribers
	b.localSubs[topic] = append(b.localSubs[topic], svc)

	// Initialize Redis pubsub if not started
	if !b.started {
		b.pubsub = b.client.PSubscribe(b.ctx, b.getTopicPattern(topic))
		b.started = true
		go b.listenForMessages()
	} else {
		// Add new pattern subscription
		if err := b.pubsub.PSubscribe(b.ctx, b.getTopicPattern(topic)); err != nil {
			return fmt.Errorf("failed to subscribe to topic pattern: %w", err)
		}
	}

	return nil
}

// GetSubscribers returns local subscribers for the given topic (used by manager for local dispatch).
// For Redis backend, this returns LOCAL subscribers only. Remote subscribers on other instances
// will receive events via Redis pub/sub (handled by handleRedisMessage).
func (b *redisBackend) GetSubscribers(topic string) []common.Named {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var subscribers []common.Named

	// Find all local subscribers that match this topic hierarchically
	for subTopic, subs := range b.localSubs {
		if b.topicMatches(subTopic, topic) {
			subscribers = append(subscribers, subs...)
		}
	}

	return subscribers
}

// PublishToRedis sends an event to Redis for distribution to other instances.
// This should be called by the manager after local dispatch.
func (b *redisBackend) PublishToRedis(svc common.Named, topic string, e common.Event) error {
	if e == nil {
		return nil
	}

	publisherName := ""
	if svc != nil {
		publisherName = svc.Name()
	}

	msg := eventMessage{
		Publisher: publisherName,
		Topic:     topic,
		Kind:      e.Kind(),
		Payload:   e,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to Redis for the exact topic and all parent topics
	sections := strings.Split(topic, "/")
	for i := range sections {
		prefix := strings.Join(sections[:i+1], "/")
		channel := b.getRedisChannel(prefix)
		if err := b.client.Publish(b.ctx, channel, data).Err(); err != nil {
			b.log.Error("failed to publish to Redis", "channel", channel, "error", err)
		}
	}

	return nil
}

// SetDispatcher sets the dispatch function used to deliver events to subscribers.
func (b *redisBackend) SetDispatcher(dispatch DispatchFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dispatch = dispatch
}

// Unsubscribe removes a service from receiving events for the given topic.
func (b *redisBackend) Unsubscribe(svc common.Named, topic string) error {
	if svc == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	subscribers, ok := b.localSubs[topic]
	if !ok {
		return nil
	}

	// Remove from local subscribers
	filtered := make([]common.Named, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub != svc {
			filtered = append(filtered, sub)
		}
	}

	if len(filtered) > 0 {
		b.localSubs[topic] = filtered
	} else {
		delete(b.localSubs, topic)
		// If no more local subscribers, unsubscribe from Redis pattern
		if b.pubsub != nil {
			pattern := b.getTopicPattern(topic)
			if err := b.pubsub.PUnsubscribe(b.ctx, pattern); err != nil {
				b.log.Error("failed to unsubscribe from pattern", "pattern", pattern, "error", err)
			}
		}
	}

	return nil
}

// Close cleans up Redis connections and resources.
func (b *redisBackend) Close() error {
	b.cancel()

	if b.pubsub != nil {
		if err := b.pubsub.Close(); err != nil {
			return fmt.Errorf("failed to close pubsub: %w", err)
		}
	}

	return nil
}

// listenForMessages continuously listens for messages from Redis and dispatches to local subscribers.
func (b *redisBackend) listenForMessages() {
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

// handleRedisMessage processes a message received from Redis.
func (b *redisBackend) handleRedisMessage(msg *redis.Message) {
	var eventMsg eventMessage
	if err := json.Unmarshal([]byte(msg.Payload), &eventMsg); err != nil {
		b.log.Error("failed to unmarshal event message", "error", err)
		return
	}

	// Find all matching local subscribers
	b.mu.RLock()
	var subscribers []common.Named
	for topic, subs := range b.localSubs {
		// Check if this topic matches the event topic (hierarchical matching)
		if b.topicMatches(topic, eventMsg.Topic) {
			subscribers = append(subscribers, subs...)
		}
	}
	dispatchFunc := b.dispatch
	b.mu.RUnlock()

	// Create a generic event wrapper
	event := &genericEvent{
		kind:    eventMsg.Kind,
		payload: eventMsg.Payload,
	}

	// Use the manager's dispatch function to deliver the event
	if dispatchFunc != nil {
		dispatchFunc(subscribers, eventMsg.Topic, event)
	}
}

// topicMatches checks if a subscription topic matches an event topic.
// Hierarchical: subscribing to "app" matches events from "app/module/component"
func (b *redisBackend) topicMatches(subTopic, eventTopic string) bool {
	// Exact match
	if subTopic == eventTopic {
		return true
	}

	// Hierarchical match: subscription is a prefix of event topic
	return strings.HasPrefix(eventTopic, subTopic+"/")
}

// getRedisChannel returns the Redis channel name for a topic.
func (b *redisBackend) getRedisChannel(topic string) string {
	return "pubsub:" + topic
}

// getTopicPattern returns the Redis pattern for subscribing to a topic and all subtopics.
func (b *redisBackend) getTopicPattern(topic string) string {
	// Pattern matches the exact topic and all subtopics
	return "pubsub:" + topic + "*"
}

// genericEvent is a simple implementation of common.Event for deserializing from Redis.
type genericEvent struct {
	kind    string
	payload any
}

func (e *genericEvent) Kind() string {
	return e.kind
}
