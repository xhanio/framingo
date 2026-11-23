package pubsub

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func newTestManager() *manager {
	b := driver.NewMemory(log.Default)
	m := newManager(b, WithLogger(log.Default), WithName("test-pubsub"))
	_ = m.Init(context.Background())
	return m
}

func drain(t *testing.T, ch <-chan entity.PubsubMessage, timeout time.Duration) []entity.PubsubMessage {
	t.Helper()
	var msgs []entity.PubsubMessage
	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		case <-deadline:
			return msgs
		}
	}
}

func TestManagerInit(t *testing.T) {
	b := driver.NewMemory(log.Default)
	m := newManager(b, WithName("test"))
	err := m.Init(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, m.bus)
}

func TestManagerStartStop(t *testing.T) {
	m := newTestManager()

	err := m.Start(context.Background())
	require.NoError(t, err)

	err = m.Stop(context.Background())
	require.NoError(t, err)
}

func TestManagerName(t *testing.T) {
	m := newManager(driver.NewMemory(log.Default), WithName("my-pubsub"))
	assert.Equal(t, "my-pubsub", m.Name())
}

func TestManagerDependencies(t *testing.T) {
	m := newTestManager()
	assert.Nil(t, m.Dependencies())
}

func TestManagerPublishSubscribe(t *testing.T) {
	m := newTestManager()

	ch, err := m.Subscribe("subscriber", "test/topic")
	require.NoError(t, err)

	err = m.Publish(context.Background(), "publisher", "test/topic", "test", "payload")
	require.NoError(t, err)

	msgs := drain(t, ch, 100*time.Millisecond)
	require.Len(t, msgs, 1)
	assert.Equal(t, "publisher", msgs[0].From)
	assert.Equal(t, "test/topic", msgs[0].Topic)
	assert.Equal(t, "test", msgs[0].Kind)
	assert.Equal(t, "payload", msgs[0].Payload)
}

func TestManagerSkipSelfDelivery(t *testing.T) {
	m := newTestManager()

	chA, err := m.Subscribe("serviceA", "topic")
	require.NoError(t, err)
	chB, err := m.Subscribe("serviceB", "topic")
	require.NoError(t, err)

	err = m.Publish(context.Background(), "serviceA", "topic", "kind", "payload")
	require.NoError(t, err)

	assert.Len(t, drain(t, chA, 100*time.Millisecond), 0, "publisher should not receive its own message")
	assert.Len(t, drain(t, chB, 100*time.Millisecond), 1, "other subscriber should receive the message")
}

func TestManagerHierarchicalTopics(t *testing.T) {
	m := newTestManager()

	rootCh, err := m.Subscribe("root", "app")
	require.NoError(t, err)
	childCh, err := m.Subscribe("child", "app/module")
	require.NoError(t, err)
	leafCh, err := m.Subscribe("leaf", "app/module/component")
	require.NoError(t, err)

	err = m.Publish(context.Background(), "publisher", "app/module/component", "leaf", nil)
	require.NoError(t, err)

	assert.Len(t, drain(t, rootCh, 100*time.Millisecond), 1)
	assert.Len(t, drain(t, childCh, 100*time.Millisecond), 1)
	assert.Len(t, drain(t, leafCh, 100*time.Millisecond), 1)

	err = m.Publish(context.Background(), "publisher", "app", "root", nil)
	require.NoError(t, err)

	assert.Len(t, drain(t, rootCh, 100*time.Millisecond), 1)
	assert.Len(t, drain(t, childCh, 100*time.Millisecond), 0)
	assert.Len(t, drain(t, leafCh, 100*time.Millisecond), 0)
}

func TestManagerUnsubscribe(t *testing.T) {
	m := newTestManager()

	ch, err := m.Subscribe("subscriber", "topic")
	require.NoError(t, err)

	require.NoError(t, m.Publish(context.Background(), "publisher", "topic", "before", nil))
	assert.Len(t, drain(t, ch, 100*time.Millisecond), 1)

	require.NoError(t, m.Unsubscribe("subscriber", "topic"))

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after Unsubscribe")

	require.NoError(t, m.Publish(context.Background(), "publisher", "topic", "after", nil))
}

func TestManagerEmptyNameSubscribeNoop(t *testing.T) {
	m := newTestManager()

	ch, err := m.Subscribe("", "topic")
	require.NoError(t, err)
	assert.Nil(t, ch, "empty name should yield no channel")

	require.NoError(t, m.Unsubscribe("", "topic"))
}

func TestManagerConcurrent(t *testing.T) {
	m := newTestManager()

	const numSubscribers = 5
	const numMessages = 50

	channels := make([]<-chan entity.PubsubMessage, numSubscribers)
	for i := range channels {
		name := "sub-" + string(rune('a'+i))
		ch, err := m.Subscribe(name, "concurrent")
		require.NoError(t, err)
		channels[i] = ch
	}

	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Publish(context.Background(), "publisher", "concurrent", "k", nil)
		}()
	}
	wg.Wait()

	for _, ch := range channels {
		msgs := drain(t, ch, 500*time.Millisecond)
		assert.Len(t, msgs, numMessages)
	}
}

func TestManagerStats(t *testing.T) {
	m := newTestManager()

	require.NoError(t, m.Publish(context.Background(), "publisher", "stats", "test", nil))
	assert.Equal(t, uint64(1), m.published.Load())
}

func TestManagerInfo(t *testing.T) {
	m := newTestManager()

	var buf bytes.Buffer
	m.Info(&buf, false)

	output := buf.String()
	assert.Contains(t, output, "test-pubsub")
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "published")
}

func TestManagerGracefulShutdown(t *testing.T) {
	m := newTestManager()

	ch, err := m.Subscribe("subscriber", "topic")
	require.NoError(t, err)

	require.NoError(t, m.Publish(context.Background(), "publisher", "topic", "test", nil))
	assert.Len(t, drain(t, ch, 100*time.Millisecond), 1)

	require.NoError(t, m.Stop(context.Background()))
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after Stop")
}

func TestManagerInfoReportsDeliveryStats(t *testing.T) {
	m := newManager(driver.NewMemory(log.Default), WithName("my-pubsub"))

	var buf bytes.Buffer
	m.Info(&buf, true)

	out := buf.String()
	assert.Contains(t, out, "dropped")
	assert.Contains(t, out, "evicted")
}

// failingDriver is a memory driver whose Publish fails as a backend would.
type failingDriver struct {
	driver.Driver
	err error
}

func (d *failingDriver) Publish(context.Context, string, string, string, any) error { return d.err }

// A failed publish is returned with its topic, for the caller's one report of it.
func TestManagerPublishFailureCarriesTheTopic(t *testing.T) {
	backend := errors.Newf("redis: connection refused")
	m := newManager(&failingDriver{Driver: driver.NewMemory(log.Default), err: backend})
	err := m.Publish(context.Background(), "a", "orders/created", "kind", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, backend)
	assert.Equal(t, "publish to topic orders/created: redis: connection refused", err.Error())
}
