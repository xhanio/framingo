package driver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// drainUntilClosed reports whether ch reaches a closed state within d, draining
// whatever is buffered in it. A subscriber that is never evicted keeps its
// channel open, so this returns false.
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

// A subscriber that stops reading must be evicted, not silently skipped.
func TestMemoryDropSubscriberEvictsLaggard(t *testing.T) {
	b := NewMemory(log.Default, WithOnFull(DropSubscriber), WithChannelBuffer(1), WithQueueCap(2))

	ch, err := b.Subscribe("slow", "topic")
	require.NoError(t, err)

	// Never read from ch, so the channel buffer and then the pending queue fill up.
	for i := 0; i < 50; i++ {
		require.NoError(t, b.Publish(context.Background(), "pub", "topic", "kind", i))
	}

	assert.True(t, drainUntilClosed(ch, 2*time.Second),
		"laggard subscriber should have been evicted and its channel closed")

	assert.Eventually(t, func() bool {
		return len(b.GetSubscribers("topic")) == 0
	}, 2*time.Second, 10*time.Millisecond, "evicted subscriber should be gone from the topic")
}

// Eviction must use the topic the subscriber actually subscribed to, not the
// topic the message was published to. A subscriber on "app" receives messages
// published to "app/module/component"; looking it up under the publish topic
// finds nothing, and the eviction silently does nothing.
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

// Evicting one laggard must not disturb a healthy subscriber on the same topic.
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

	// Publish slowly enough that a subscriber which is actually draining stays
	// well inside its queue; only the one that never reads falls behind.
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

// The default policy stays DropMessage for backward compatibility, but the
// drops must now be counted rather than silently swallowed.
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

	// The subscriber is kept, because DropMessage drops the message, not the peer.
	assert.Len(t, b.GetSubscribers("topic"), 1)
}

// A healthy subscriber loses nothing and nothing is counted as dropped.
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

// The pending queue exists to absorb a burst that overruns the channel buffer.
// A subscriber that reads late, but does read, must lose nothing and stay
// subscribed, and the messages must arrive in publish order.
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

// Unsubscribing an already-evicted subscriber must not panic on a double close.
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
		_ = b.Stop(true)
	})
}
