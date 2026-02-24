package driver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	defer b.Stop(true)

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
	defer b.Stop(true)

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
	defer b.Stop(true)

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
		_ = b1.Stop(true)
	}()
	defer func() {
		_ = b2.Stop(true)
	}()

	// Give Kafka time to set up consumer groups
	time.Sleep(5 * time.Second)

	err = b1.Publish("publisher", "cross/topic", "cross-event", map[string]string{"key": "value"})
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Equal(t, "cross-event", msg.Kind)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for cross-instance message")
	}
}

func TestKafkaStartStop(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	err = b.Start(context.Background())
	require.NoError(t, err)

	err = b.Stop(true)
	require.NoError(t, err)
}

func TestKafkaStopClosesChannels(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))

	ch, _ := b.Subscribe("svc", "topic")

	err = b.Stop(true)
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

	err = b.Stop(true)
	require.NoError(t, err)

	// Second stop should not panic
	err = b.Stop(true)
	assert.NoError(t, err)
}

func TestKafkaStopWithoutStart(t *testing.T) {
	brokers := getTestKafkaBrokers(t)

	b, err := NewKafka(brokers, "test-group", log.Default)
	require.NoError(t, err)

	_, _ = b.Subscribe("svc", "topic")

	// Stop without Start should not panic
	err = b.Stop(true)
	assert.NoError(t, err)
}
