package driver

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func getTestKafkaBrokers(t *testing.T) []string {
	brokers := []string{"localhost:9092"}

	conn, err := net.DialTimeout("tcp", brokers[0], 2*time.Second)
	if err != nil {
		t.Skipf("skipping kafka tests: %v", err)
	}
	conn.Close()

	return brokers
}

func TestKafkaNewNilBrokers(t *testing.T) {
	_, err := NewKafka(nil, "test-group", log.Default)
	assert.Error(t, err)
}

func TestKafkaNewEmptyGroupID(t *testing.T) {
	_, err := NewKafka([]string{"localhost:9092"}, "", log.Default)
	assert.Error(t, err)
}

func TestKafkaSubscribeAndGet(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))
	//nolint:errcheck // test teardown; a failed stop has nothing to act on
	defer b.Stop(context.Background())

	ch, err := b.Subscribe("svc1", "test/topic")
	require.NoError(t, err)
	assert.NotNil(t, ch)

	subs := b.GetSubscribers("test/topic")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc1", subs[0])
}

func TestKafkaHierarchicalTopics(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))
	//nolint:errcheck // test teardown; a failed stop has nothing to act on
	defer b.Stop(context.Background())

	_, _ = b.Subscribe("root", "app")
	_, _ = b.Subscribe("child", "app/module")
	_, _ = b.Subscribe("leaf", "app/module/component")

	subs := b.GetSubscribers("app/module/component")
	assert.Len(t, subs, 3)

	subs = b.GetSubscribers("app/module")
	assert.Len(t, subs, 2)

	subs = b.GetSubscribers("app")
	assert.Len(t, subs, 1)
}

func TestKafkaUnsubscribe(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))
	//nolint:errcheck // test teardown; a failed stop has nothing to act on
	defer b.Stop(context.Background())

	_, _ = b.Subscribe("svc1", "topic")
	_, _ = b.Subscribe("svc2", "topic")

	err = b.Unsubscribe("svc1", "topic")
	require.NoError(t, err)

	subs := b.GetSubscribers("topic")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc2", subs[0])
}

func TestKafkaCrossInstance(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b1, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	b2, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	ch, err := b2.Subscribe("remote-subscriber", "cross/topic")
	require.NoError(t, err)

	require.NoError(t, b1.Start(context.Background()))
	require.NoError(t, b2.Start(context.Background()))

	defer func() {
		_ = b1.Stop(context.Background())
	}()
	defer func() {
		_ = b2.Stop(context.Background())
	}()

	untilDelivering(t, b1, ch, "cross/topic")

	err = b1.Publish(context.Background(), "publisher", "cross/topic", "cross-event", map[string]string{"key": "value"})
	require.NoError(t, err)

	timeout := time.After(10 * time.Second)
	for {
		select {
		case msg := <-ch:
			if msg.Kind == "probe" {
				continue // published before the subscriber was seen receiving
			}
			assert.Equal(t, "cross-event", msg.Kind)
			return
		case <-timeout:
			t.Fatal("timeout waiting for cross-instance message")
		}
	}
}

// untilDelivering publishes probes from pub until ch yields one. A new instance
// reads from the latest offset once its group holds the partition, and the
// topic exists only once first written: what is published before then never
// reaches it, so a fixed sleep is a guess.
func untilDelivering(t *testing.T, pub Driver, ch <-chan entity.PubsubMessage, topic string) {
	t.Helper()
	deadline := time.After(60 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		// the first write creates the topic, and fails doing so
		if err := pub.Publish(context.Background(), "prober", topic, "probe", nil); err != nil {
			t.Logf("probe not published: %v", err)
		}
		select {
		case msg := <-ch:
			if msg.Kind == "probe" {
				return
			}
		case <-tick.C:
		case <-deadline:
			t.Fatal("the subscriber never started receiving")
		}
	}
}

func TestKafkaStartStop(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	err = b.Start(context.Background())
	require.NoError(t, err)

	err = b.Stop(context.Background())
	require.NoError(t, err)
}

func TestKafkaStopClosesChannels(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))

	ch, _ := b.Subscribe("svc", "topic")

	err = b.Stop(context.Background())
	require.NoError(t, err)

	_, ok := <-ch
	assert.False(t, ok)
}

func TestKafkaDoubleStop(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))

	_, _ = b.Subscribe("svc", "topic")

	err = b.Stop(context.Background())
	require.NoError(t, err)

	err = b.Stop(context.Background())
	assert.NoError(t, err)
}

func TestKafkaStopWithoutStart(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	_, _ = b.Subscribe("svc", "topic")

	err = b.Stop(context.Background())
	assert.NoError(t, err)
}

// an event carrying this instance's origin was delivered by Publish already;
// one from another instance is delivered
func TestKafkaSkipsItsOwnEcho(t *testing.T) {
	b := &kafkaDriver{dispatcher: newDispatcher(log.Default), topics: make(map[string][]*subscriber), origin: "me"}
	ch, err := b.Subscribe("local", "echo/topic")
	require.NoError(t, err)
	event := func(origin string) []byte {
		data, err := json.Marshal(eventMessage{Origin: origin, Publisher: "publisher", Topic: "echo/topic", Kind: "k", Payload: json.RawMessage(`"x"`)})
		require.NoError(t, err)
		return data
	}

	b.handleKafkaMessage(event("me"))
	select {
	case msg := <-ch:
		t.Fatalf("its own echo was delivered: %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	b.handleKafkaMessage(event("another"))
	select {
	case msg := <-ch:
		assert.Equal(t, "k", msg.Kind)
	case <-time.After(time.Second):
		t.Fatal("another instance's event was not delivered")
	}
}
