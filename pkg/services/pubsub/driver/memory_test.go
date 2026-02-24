package driver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func TestMemorySubscribeAndGet(t *testing.T) {
	b := NewMemory(log.Default)

	ch, err := b.Subscribe("svc1", "topic/a")
	require.NoError(t, err)
	assert.NotNil(t, ch)

	subs := b.GetSubscribers("topic/a")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc1", subs[0])
}

func TestMemoryHierarchicalTopics(t *testing.T) {
	b := NewMemory(log.Default)

	_, _ = b.Subscribe("root", "app")
	_, _ = b.Subscribe("child", "app/module")
	_, _ = b.Subscribe("leaf", "app/module/component")

	// Publishing to leaf should return all three
	subs := b.GetSubscribers("app/module/component")
	assert.Len(t, subs, 3)

	// Publishing to middle should return root and child
	subs = b.GetSubscribers("app/module")
	assert.Len(t, subs, 2)

	// Publishing to root should return only root
	subs = b.GetSubscribers("app")
	assert.Len(t, subs, 1)
}

func TestMemoryMultipleSubscribers(t *testing.T) {
	b := NewMemory(log.Default)

	_, _ = b.Subscribe("svc1", "topic")
	_, _ = b.Subscribe("svc2", "topic")

	subs := b.GetSubscribers("topic")
	assert.Len(t, subs, 2)
}

func TestMemoryPublish(t *testing.T) {
	b := NewMemory(log.Default)

	ch, err := b.Subscribe("sub", "topic")
	require.NoError(t, err)

	err = b.Publish("pub", "topic", "test-kind", "test-payload")
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Equal(t, "pub", msg.From)
		assert.Equal(t, "topic", msg.Topic)
		assert.Equal(t, "test-kind", msg.Kind)
		assert.Equal(t, "test-payload", msg.Payload)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryPublishSkipSelf(t *testing.T) {
	b := NewMemory(log.Default)

	ch, _ := b.Subscribe("svc", "topic")

	_ = b.Publish("svc", "topic", "test", nil)

	select {
	case <-ch:
		t.Fatal("should not receive own message")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestMemoryUnsubscribe(t *testing.T) {
	b := NewMemory(log.Default)

	_, _ = b.Subscribe("svc1", "topic")
	ch2, _ := b.Subscribe("svc2", "topic")

	err := b.Unsubscribe("svc1", "topic")
	require.NoError(t, err)

	subs := b.GetSubscribers("topic")
	assert.Len(t, subs, 1)
	assert.Equal(t, "svc2", subs[0])

	// svc2's channel should still work
	_ = b.Publish("pub", "topic", "test", nil)
	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message on remaining subscriber")
	}
}

func TestMemoryUnsubscribeLast(t *testing.T) {
	b := NewMemory(log.Default)

	ch, _ := b.Subscribe("svc", "topic")

	err := b.Unsubscribe("svc", "topic")
	require.NoError(t, err)

	subs := b.GetSubscribers("topic")
	assert.Len(t, subs, 0)

	// channel should be closed
	_, ok := <-ch
	assert.False(t, ok)
}

func TestMemoryEmptyName(t *testing.T) {
	b := NewMemory(log.Default)

	ch, err := b.Subscribe("", "topic")
	assert.NoError(t, err)
	assert.Nil(t, ch)

	err = b.Unsubscribe("", "topic")
	assert.NoError(t, err)
}

func TestMemoryNoSubscribers(t *testing.T) {
	b := NewMemory(log.Default)
	subs := b.GetSubscribers("nonexistent")
	assert.Len(t, subs, 0)
}

func TestMemoryStartStop(t *testing.T) {
	b := NewMemory(log.Default)
	assert.NoError(t, b.Start(context.Background()))
	assert.NoError(t, b.Stop(true))
}

func TestMemoryUnsubscribeNonexistent(t *testing.T) {
	b := NewMemory(log.Default)

	err := b.Unsubscribe("svc", "nonexistent")
	assert.NoError(t, err)
}

func TestMemoryStopClosesChannels(t *testing.T) {
	b := NewMemory(log.Default)

	ch, _ := b.Subscribe("svc", "topic")

	err := b.Stop(true)
	require.NoError(t, err)

	// channel should be closed
	_, ok := <-ch
	assert.False(t, ok)
}

func TestMemoryDoubleStop(t *testing.T) {
	b := NewMemory(log.Default)

	_, _ = b.Subscribe("svc", "topic")

	err := b.Stop(true)
	require.NoError(t, err)

	// Second stop should not panic
	err = b.Stop(true)
	assert.NoError(t, err)
}
