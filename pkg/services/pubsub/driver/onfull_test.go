package driver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func drainUntilClosed[T any](ch <-chan T, d time.Duration) bool {
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for range ch {
		}
	}()
	select {
	case <-closed:
		return true
	case <-time.After(d):
		return false
	}
}

func dropped(t *testing.T, d Driver) uint64 {
	t.Helper()
	s, ok := d.(Stats)
	require.True(t, ok, "driver should report delivery stats")
	return s.Dropped()
}

func TestMemoryDropSubscriberEvictsLaggard(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(2))

	ch, err := b.Subscribe("slow", "topic")
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
	}

	assert.True(t, drainUntilClosed(ch, 2*time.Second),
		"laggard subscriber should have been evicted and its channel closed")

	assert.Eventually(t, func() bool {
		return len(b.GetSubscribers("topic")) == 0
	}, 2*time.Second, 10*time.Millisecond, "evicted subscriber should be gone from the topic")
}

func TestMemoryDropSubscriberEvictsPrefixSubscriber(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(2))

	ch, err := b.Subscribe("slow", "app")
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "app/module/component", "kind", i))
	}

	assert.True(t, drainUntilClosed(ch, 2*time.Second),
		"subscriber on a parent topic should be evicted when it lags")

	assert.Eventually(t, func() bool {
		return len(b.GetSubscribers("app")) == 0
	}, 2*time.Second, 10*time.Millisecond, "evicted subscriber should be gone from its subscription topic")
}

func TestMemoryDropSubscriberSparesHealthySubscriber(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(2), WithQueueCap(8))

	slow, err := b.Subscribe("slow", "topic")
	require.NoError(t, err)
	fast, err := b.Subscribe("fast", "topic")
	require.NoError(t, err)

	go func() {
		for range fast {
		}
	}()

	// slow enough that a draining subscriber stays well inside its queue
	for i := 0; i < 100; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
		time.Sleep(200 * time.Microsecond)
	}

	assert.True(t, drainUntilClosed(slow, 2*time.Second), "slow subscriber should be evicted")

	assert.Eventually(t, func() bool {
		subs := b.GetSubscribers("topic")
		return len(subs) == 1 && subs[0] == "fast"
	}, 2*time.Second, 10*time.Millisecond, "healthy subscriber should survive")
}

func TestMemoryDropMessageCountsDrops(t *testing.T) {
	b := NewMemory(log.Default, WithChannelBuffer(1), WithQueueCap(2))

	_, err := b.Subscribe("slow", "topic")
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
	}

	assert.Eventually(t, func() bool {
		return dropped(t, b) > 0
	}, 2*time.Second, 10*time.Millisecond, "dropped messages should be counted")

	assert.Len(t, b.GetSubscribers("topic"), 1)
}

func TestMemoryNoDropsForHealthySubscriber(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(2))

	ch, err := b.Subscribe("sub", "topic")
	require.NoError(t, err)

	const n = 100
	for i := 0; i < n; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
		select {
		case msg := <-ch:
			assert.Equal(t, i, msg.Payload)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}

	assert.Zero(t, dropped(t, b))
	assert.Len(t, b.GetSubscribers("topic"), 1)
}

func TestMemoryQueueAbsorbsBurstBeyondChannelBuffer(t *testing.T) {
	const burst = 100
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(burst))

	ch, err := b.Subscribe("late", "topic")
	require.NoError(t, err)

	for i := 0; i < burst; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
	}

	for i := 0; i < burst; i++ {
		select {
		case msg, ok := <-ch:
			require.True(t, ok, "subscriber was evicted after %d of %d messages", i, burst)
			assert.Equal(t, i, msg.Payload, "messages should arrive in publish order")
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d of %d", i, burst)
		}
	}

	assert.Zero(t, dropped(t, b))
	assert.Len(t, b.GetSubscribers("topic"), 1)
}

func TestMemoryEvictThenUnsubscribeDoesNotPanic(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(2))

	ch, err := b.Subscribe("slow", "topic")
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
	}
	require.True(t, drainUntilClosed(ch, 2*time.Second))

	assert.NotPanics(t, func() {
		_ = b.Unsubscribe("slow", "topic")
		_ = b.Stop(context.Background())
	})
}
