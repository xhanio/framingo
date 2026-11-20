package driver

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

type kafkaDriver struct {
	*dispatcher

	kafkaTopic string // one Kafka topic carries every pubsub topic

	writer *kafka.Writer
	reader *kafka.Reader

	mu     sync.RWMutex
	topics map[string][]*subscriber

	origin string // this instance, on every event it publishes

	loop syncutil.Loop // consumes the Kafka topic, from Start to Stop
}

func NewKafka(brokers []string, groupID string, log log.Logger, opts ...Option) (Driver, error) {
	if len(brokers) == 0 {
		return nil, errors.Newf("at least one broker address is required")
	}
	if groupID == "" {
		return nil, errors.Newf("group ID cannot be empty")
	}

	// a unique group per instance: a shared group would partition messages instead of broadcasting
	origin := uuid.New().String()
	instanceGroupID := groupID + "-" + origin

	kafkaTopic := "pubsub"

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  kafkaTopic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     instanceGroupID,
		Topic:       kafkaTopic,
		StartOffset: kafka.LastOffset,
	})

	return &kafkaDriver{
		dispatcher: newDispatcher(log, opts...),
		kafkaTopic: kafkaTopic,
		writer:     writer,
		reader:     reader,
		topics:     make(map[string][]*subscriber),
		origin:     origin,
	}, nil
}

func (b *kafkaDriver) Subscribe(name string, topic string) (<-chan entity.PubsubMessage, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	sub := newSubscriber(name, b.opts)
	b.topics[topic] = append(b.topics[topic], sub)

	return sub.ch, nil
}

func (b *kafkaDriver) GetSubscribers(topic string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.getSubscribers(topic)
	names := make([]string, len(subs))
	for i, sub := range subs {
		names[i] = sub.name
	}
	return names
}

func (b *kafkaDriver) getSubscribers(topic string) []*subscriber {
	var subscribers []*subscriber
	for subTopic, subs := range b.topics {
		if topicMatches(subTopic, topic) {
			subscribers = append(subscribers, subs...)
		}
	}
	return subscribers
}

func (b *kafkaDriver) Unsubscribe(name string, topic string) error {
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

	if len(filtered) > 0 {
		b.topics[topic] = filtered
	} else {
		delete(b.topics, topic)
	}
	b.mu.Unlock()

	// outside the lock: the pump owns close(ch)
	for _, sub := range removed {
		sub.stop()
	}
	return nil
}

func (b *kafkaDriver) evict(lagged []laggard) {
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

// by identity, not name: a same-name resubscribe can land between RUnlock and Lock
func (b *kafkaDriver) remove(topic string, target *subscriber) {
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
	} else {
		delete(b.topics, topic)
	}
}

func (b *kafkaDriver) Publish(ctx context.Context, from string, topic string, kind string, payload any) error {
	msg := entity.PubsubMessage{From: from, Topic: topic, Kind: kind, Payload: payload}

	b.mu.RLock()
	lagged := b.fanout(b.topics, from, msg)
	b.mu.RUnlock()

	// after RUnlock: RWMutex is not upgradable
	b.evict(lagged)

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal event payload")
	}

	em := eventMessage{
		Origin:    b.origin,
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

	return b.writer.WriteMessages(ctx, kafka.Message{Value: data})
}

func (b *kafkaDriver) Start(ctx context.Context) error {
	b.loop.Start(ctx, b.consume)
	return nil
}

// Stop ignores ctx: the listener and pumps end as soon as they are stopped.
func (b *kafkaDriver) Stop(_ context.Context) error {
	b.loop.Stop()

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

	for _, sub := range stopped {
		sub.wait()
	}

	werr := b.writer.Close()
	rerr := b.reader.Close()

	return errors.Combine(werr, rerr)
}

func (b *kafkaDriver) consume(ctx context.Context) {
	for {
		msg, err := b.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Errorf("kafka read error: %v", err)
			continue
		}
		b.handleKafkaMessage(msg.Value)
	}
}

func (b *kafkaDriver) handleKafkaMessage(data []byte) {
	var em eventMessage
	if err := json.Unmarshal(data, &em); err != nil {
		b.log.Errorf("failed to unmarshal kafka event: %v", err)
		return
	}
	if em.Origin == b.origin {
		return // Publish delivered it locally already
	}

	m := entity.PubsubMessage{
		From:    em.Publisher,
		Topic:   em.Topic,
		Kind:    em.Kind,
		Payload: em.Payload,
	}

	b.mu.RLock()
	lagged := b.fanout(b.topics, em.Publisher, m)
	b.mu.RUnlock()

	b.evict(lagged)
}
