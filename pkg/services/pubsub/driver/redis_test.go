package driver

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func getTestRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // use DB 15 for testing
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping redis tests: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

func TestRedisNewNilClient(t *testing.T) {
	_, err := NewRedis(nil, log.Default)
	assert.Error(t, err)
}

func TestRedisSubscribeAndGet(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)
	defer b.Stop(true)

	ch, err := b.Subscribe("svc1", "test/topic")
	require.NoError(t, err)
	assert.NotNil(t, ch)

	subs := b.GetSubscribers("test/topic")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc1", subs[0])
}

func TestRedisHierarchicalTopics(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)
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

func TestRedisUnsubscribe(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)
	defer b.Stop(true)

	_, _ = b.Subscribe("svc1", "topic")
	_, _ = b.Subscribe("svc2", "topic")

	err = b.Unsubscribe("svc1", "topic")
	require.NoError(t, err)

	subs := b.GetSubscribers("topic")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc2", subs[0])
}

func TestRedisCrossInstance(t *testing.T) {
	client := getTestRedisClient(t)

	// Create two backends simulating two instances
	b1, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	b2, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	ch, err := b2.Subscribe("remote-subscriber", "cross/topic")
	require.NoError(t, err)

	// Start both backends
	require.NoError(t, b1.Start(context.Background()))
	require.NoError(t, b2.Start(context.Background()))

	defer b1.Stop(true)
	defer b2.Stop(true)

	// Give Redis time to set up subscriptions
	time.Sleep(200 * time.Millisecond)

	// Publish via b1
	err = b1.Publish("publisher", "cross/topic", "cross-event", map[string]string{"key": "value"})
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Equal(t, "cross-event", msg.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cross-instance message")
	}
}

func TestRedisStartStop(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	err = b.Start(context.Background())
	require.NoError(t, err)

	err = b.Stop(true)
	require.NoError(t, err)
}

func TestRedisStopClosesChannels(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))

	ch, _ := b.Subscribe("svc", "topic")

	err = b.Stop(true)
	require.NoError(t, err)

	_, ok := <-ch
	assert.False(t, ok)
}

func TestRedisDoubleStop(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	require.NoError(t, b.Start(context.Background()))

	_, _ = b.Subscribe("svc", "topic")

	err = b.Stop(true)
	require.NoError(t, err)

	// Second stop should not panic
	err = b.Stop(true)
	assert.NoError(t, err)
}

func TestRedisStopWithoutStart(t *testing.T) {
	client := getTestRedisClient(t)

	b, err := NewRedis(client, log.Default)
	require.NoError(t, err)

	_, _ = b.Subscribe("svc", "topic")

	// Stop without Start should not panic
	err = b.Stop(true)
	assert.NoError(t, err)
}
