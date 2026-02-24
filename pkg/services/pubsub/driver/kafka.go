package driver

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type kafkaDriver struct {
	log log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	kafkaTopic string // single Kafka topic for all pubsub messages

	writer *kafka.Writer
	reader *kafka.Reader

	mu     sync.RWMutex
	topics map[string][]subscriber // pubsub topic -> local subscribers

	wg sync.WaitGroup
}

func NewKafka(brokers []string, groupID string, log log.Logger) (Driver, error) {
	if len(brokers) == 0 {
		return nil, errors.Newf("at least one broker address is required")
	}
	if groupID == "" {
		return nil, errors.Newf("group ID cannot be empty")
	}

	// Each instance gets a unique consumer group for broadcast semantics.
	// Kafka consumer groups partition messages across members;
	// unique groups ensure every instance receives all messages.
	instanceGroupID := groupID + "-" + uuid.New().String()

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
		log:        log,
		kafkaTopic: kafkaTopic,
		writer:     writer,
		reader:     reader,
		topics:     make(map[string][]subscriber),
	}, nil
}

func (b *kafkaDriver) Subscribe(name string, topic string) (<-chan Message, error) {
	if name == "" {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, channelBufferSize)
	sub := subscriber{name: name, ch: ch}
	b.topics[topic] = append(b.topics[topic], sub)

	return ch, nil
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

func (b *kafkaDriver) getSubscribers(topic string) []subscriber {
	var subscribers []subscriber
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
	}

	return nil
}

func (b *kafkaDriver) Publish(from string, topic string, kind string, payload any) error {
	// Local delivery
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

	// Remote delivery via Kafka
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

	return b.writer.WriteMessages(ctx, kafka.Message{Value: data})
}

func (b *kafkaDriver) Start(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.consumeLoop()
	return nil
}

func (b *kafkaDriver) Stop(wait bool) error {
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

	werr := b.writer.Close()
	rerr := b.reader.Close()

	return errors.Combine(werr, rerr)
}

func (b *kafkaDriver) consumeLoop() {
	defer b.wg.Done()

	for {
		msg, err := b.reader.ReadMessage(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
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

	b.mu.RLock()
	defer b.mu.RUnlock()

	m := Message{
		From:    em.Publisher,
		Topic:   em.Topic,
		Kind:    em.Kind,
		Payload: em.Payload,
	}

	for subTopic, subs := range b.topics {
		if topicMatches(subTopic, em.Topic) {
			for _, sub := range subs {
				if em.Publisher != "" && sub.name == em.Publisher {
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
