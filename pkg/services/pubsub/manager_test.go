package pubsub

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type mockMessage struct {
	kind string
}

func (e *mockMessage) Kind() string {
	return e.kind
}

type mockService struct {
	name   string
	events []common.Message
	mu     sync.Mutex
}

func (s *mockService) Name() string {
	return s.name
}

func (s *mockService) HandleMessage(e common.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *mockService) GetMessages() []common.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]common.Message, len(s.events))
	copy(events, s.events)
	return events
}

type mockRawService struct {
	name     string
	rawKinds []string
	payloads []any
	mu       sync.Mutex
}

func (s *mockRawService) Name() string {
	return s.name
}

func (s *mockRawService) HandleRawMessage(kind string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawKinds = append(s.rawKinds, kind)
	s.payloads = append(s.payloads, payload)
	return nil
}

func (s *mockRawService) GetRawMessages() ([]string, []any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kinds := make([]string, len(s.rawKinds))
	payloads := make([]any, len(s.payloads))
	copy(kinds, s.rawKinds)
	copy(payloads, s.payloads)
	return kinds, payloads
}

type mockDualService struct {
	name     string
	events   []common.Message
	rawKinds []string
	payloads []any
	mu       sync.Mutex
}

func (s *mockDualService) Name() string {
	return s.name
}

func (s *mockDualService) HandleMessage(e common.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *mockDualService) HandleRawMessage(kind string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawKinds = append(s.rawKinds, kind)
	s.payloads = append(s.payloads, payload)
	return nil
}

func newTestManager() *manager {
	b := driver.NewMemory(log.Default)
	m := newManager(b, WithLogger(log.Default), WithName("test-pubsub"))
	_ = m.Init()
	return m
}

func TestManagerInit(t *testing.T) {
	b := driver.NewMemory(log.Default)
	m := newManager(b, WithName("test"))
	err := m.Init()
	require.NoError(t, err)
	assert.NotNil(t, m.bus)
}

func TestManagerStartStop(t *testing.T) {
	m := newTestManager()

	err := m.Start(context.Background())
	require.NoError(t, err)

	// Double start should be no-op
	err = m.Start(context.Background())
	require.NoError(t, err)

	err = m.Stop(true)
	require.NoError(t, err)

	// Double stop should be no-op
	err = m.Stop(true)
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
	svc := &mockService{name: "subscriber"}

	m.Subscribe(svc, "test/topic")

	publisher := &mockService{name: "publisher"}
	m.Publish(publisher, "test/topic", "test", &mockMessage{kind: "test"})

	time.Sleep(50 * time.Millisecond)

	events := svc.GetMessages()
	assert.Len(t, events, 1)
	assert.Equal(t, "test", events[0].Kind())
}

func TestManagerSkipSelfDelivery(t *testing.T) {
	m := newTestManager()

	svcA := &mockService{name: "serviceA"}
	svcB := &mockService{name: "serviceB"}

	m.Subscribe(svcA, "topic")
	m.Subscribe(svcB, "topic")

	// serviceA publishes - should NOT receive its own event
	m.Publish(svcA, "topic", "test", &mockMessage{kind: "test"})

	time.Sleep(50 * time.Millisecond)

	assert.Len(t, svcA.GetMessages(), 0, "publisher should not receive its own event")
	assert.Len(t, svcB.GetMessages(), 1, "other subscriber should receive the event")
}

func TestManagerHierarchicalTopics(t *testing.T) {
	m := newTestManager()

	root := &mockService{name: "root-sub"}
	child := &mockService{name: "child-sub"}
	leaf := &mockService{name: "leaf-sub"}

	m.Subscribe(root, "app")
	m.Subscribe(child, "app/module")
	m.Subscribe(leaf, "app/module/component")

	publisher := &mockService{name: "publisher"}

	// Publishing to leaf topic should notify all ancestor subscribers
	m.Publish(publisher, "app/module/component", "leaf-event", &mockMessage{kind: "leaf-event"})

	time.Sleep(50 * time.Millisecond)

	assert.Len(t, root.GetMessages(), 1)
	assert.Len(t, child.GetMessages(), 1)
	assert.Len(t, leaf.GetMessages(), 1)

	// Publishing to "app" should only notify root subscriber
	m.Publish(publisher, "app", "root-event", &mockMessage{kind: "root-event"})

	time.Sleep(50 * time.Millisecond)

	assert.Len(t, root.GetMessages(), 2)
	assert.Len(t, child.GetMessages(), 1)
	assert.Len(t, leaf.GetMessages(), 1)
}

func TestManagerSendMessage(t *testing.T) {
	m := newTestManager()

	publisher := &mockService{name: "my/service"}
	subscriber := &mockService{name: "subscriber"}

	// Subscribe to the publisher's name as topic
	m.Subscribe(subscriber, "my/service")

	m.SendMessage(context.Background(), publisher, &mockMessage{kind: "hello"})

	time.Sleep(50 * time.Millisecond)

	events := subscriber.GetMessages()
	assert.Len(t, events, 1)
	assert.Equal(t, "hello", events[0].Kind())
}

func TestManagerSendRawMessage(t *testing.T) {
	m := newTestManager()

	publisher := &mockService{name: "raw-publisher"}
	subscriber := &mockRawService{name: "raw-subscriber"}

	m.Subscribe(subscriber, "raw-publisher")

	m.SendRawMessage(context.Background(), publisher, "raw-kind", map[string]string{"key": "value"})

	time.Sleep(50 * time.Millisecond)

	kinds, payloads := subscriber.GetRawMessages()
	assert.Len(t, kinds, 1)
	assert.Equal(t, "raw-kind", kinds[0])
	assert.NotNil(t, payloads[0])
}

func TestManagerDualHandler(t *testing.T) {
	m := newTestManager()

	dual := &mockDualService{name: "dual"}
	m.Subscribe(dual, "topic")

	publisher := &mockService{name: "publisher"}

	// Publish with Event payload triggers both EventHandler and RawEventHandler
	m.Publish(publisher, "topic", "typed", &mockMessage{kind: "typed"})
	time.Sleep(50 * time.Millisecond)

	dual.mu.Lock()
	assert.Len(t, dual.events, 1)
	assert.Len(t, dual.rawKinds, 1)
	assert.Equal(t, "typed", dual.events[0].Kind())
	assert.Equal(t, "typed", dual.rawKinds[0])
	dual.mu.Unlock()

	// Publish with non-Event payload triggers RawEventHandler only
	m.Publish(publisher, "topic", "raw", "data")
	time.Sleep(50 * time.Millisecond)

	dual.mu.Lock()
	assert.Len(t, dual.events, 1)
	assert.Len(t, dual.rawKinds, 2)
	assert.Equal(t, "raw", dual.rawKinds[1])
	dual.mu.Unlock()
}

func TestManagerUnsubscribe(t *testing.T) {
	m := newTestManager()

	svc := &mockService{name: "subscriber"}
	m.Subscribe(svc, "topic")

	publisher := &mockService{name: "publisher"}
	m.Publish(publisher, "topic", "before", &mockMessage{kind: "before"})
	time.Sleep(50 * time.Millisecond)
	assert.Len(t, svc.GetMessages(), 1)

	m.Unsubscribe(svc, "topic")

	m.Publish(publisher, "topic", "after", &mockMessage{kind: "after"})
	time.Sleep(50 * time.Millisecond)
	assert.Len(t, svc.GetMessages(), 1, "should not receive events after unsubscribe")
}

func TestManagerNilSafety(t *testing.T) {
	m := newTestManager()

	// These should not panic
	m.Publish(nil, "topic", "test", &mockMessage{kind: "test"})
	m.Publish(&mockService{name: "pub"}, "topic", "test", nil)
	m.Subscribe(nil, "topic")
	m.Unsubscribe(nil, "topic")
	m.SendMessage(context.Background(), nil, &mockMessage{kind: "test"})
	m.SendMessage(context.Background(), &mockService{name: "pub"}, nil)
	m.SendRawMessage(context.Background(), nil, "kind", "payload")
}

func TestManagerConcurrent(t *testing.T) {
	m := newTestManager()

	const numSubscribers = 10
	const numEvents = 100

	subscribers := make([]*mockService, numSubscribers)
	for i := range subscribers {
		svc := &mockService{name: "subscriber"}
		subscribers[i] = svc
		m.Subscribe(svc, "concurrent")
	}

	publisher := &mockService{name: "publisher"}

	var wg sync.WaitGroup
	for i := range numEvents {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Publish(publisher, "concurrent", "event", &mockMessage{kind: "event"})
		}(i)
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	for _, sub := range subscribers {
		events := sub.GetMessages()
		assert.Len(t, events, numEvents)
	}
}

func TestManagerStats(t *testing.T) {
	m := newTestManager()

	svc := &mockService{name: "subscriber"}
	m.Subscribe(svc, "stats")

	publisher := &mockService{name: "publisher"}
	m.Publish(publisher, "stats", "test", &mockMessage{kind: "test"})

	time.Sleep(50 * time.Millisecond)

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

	svc := &mockService{name: "subscriber"}
	m.Subscribe(svc, "topic")

	publisher := &mockService{name: "publisher"}
	m.Publish(publisher, "topic", "test", &mockMessage{kind: "test"})
	time.Sleep(50 * time.Millisecond)
	assert.Len(t, svc.GetMessages(), 1)

	// Stop should close channels and wait for listeners
	err := m.Stop(true)
	require.NoError(t, err)

	// Double stop with subscribers should not panic
	err = m.Stop(true)
	require.NoError(t, err)
}
