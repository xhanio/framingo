package pubsub

import (
	"sync"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type mockEvent struct {
	kind string
}

func (e *mockEvent) Kind() string {
	return e.kind
}

type mockService struct {
	name   string
	events []common.Event
	mu     sync.Mutex
}

func (s *mockService) Name() string {
	return s.name
}

func (s *mockService) HandleEvent(e common.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *mockService) GetEvents() []common.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]common.Event, len(s.events))
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

func (s *mockRawService) HandleRawEvent(kind string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawKinds = append(s.rawKinds, kind)
	s.payloads = append(s.payloads, payload)
	return nil
}

func (s *mockRawService) GetRawEvents() ([]string, []any) {
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
	events   []common.Event
	rawKinds []string
	payloads []any
	mu       sync.Mutex
}

func (s *mockDualService) Name() string {
	return s.name
}

func (s *mockDualService) HandleEvent(e common.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *mockDualService) HandleRawEvent(kind string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rawKinds = append(s.rawKinds, kind)
	s.payloads = append(s.payloads, payload)
	return nil
}

func (s *mockDualService) GetEvents() []common.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]common.Event, len(s.events))
	copy(events, s.events)
	return events
}

func (s *mockDualService) GetRawEvents() ([]string, []any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kinds := make([]string, len(s.rawKinds))
	payloads := make([]any, len(s.payloads))
	copy(kinds, s.rawKinds)
	copy(payloads, s.payloads)
	return kinds, payloads
}

func TestManagerNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("expected manager to be created")
	}

	if m.Name() == "" {
		t.Error("expected manager to have a name")
	}
}

func TestManagerSubscribe(t *testing.T) {
	m := newManager()
	service := &mockService{name: "testservice"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	m.Subscribe(service, "test/topic")
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 1 {
		t.Errorf("expected service to receive 1 event, got %d", len(events))
	}
}

func TestManagerSubscribeMultipleServices(t *testing.T) {
	m := newManager()
	service1 := &mockService{name: "service1"}
	service2 := &mockService{name: "service2"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	m.Subscribe(service1, "test/topic")
	m.Subscribe(service2, "test/topic")
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)

	events1 := service1.GetEvents()
	events2 := service2.GetEvents()

	if len(events1) != 1 {
		t.Errorf("expected service1 to receive 1 event, got %d", len(events1))
	}

	if len(events2) != 1 {
		t.Errorf("expected service2 to receive 1 event, got %d", len(events2))
	}
}

func TestManagerSubscribeNilService(t *testing.T) {
	m := newManager()
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	// Subscribe with nil service should not panic
	m.Subscribe(nil, "test/topic")

	// Publish to verify no nil pointer issues
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)
	// Test passes if no panic occurs
}

func TestManagerPublishEventHandler(t *testing.T) {
	m := newManager()
	service := &mockService{name: "testservice"}
	event := &mockEvent{kind: "testevent"}
	publisher := &mockService{name: "publisher"}

	m.Subscribe(service, "test/topic")
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	if events[0].Kind() != "testevent" {
		t.Errorf("expected event kind 'testevent', got '%s'", events[0].Kind())
	}
}

func TestManagerPublishRawEventHandler(t *testing.T) {
	m := newManager()
	service := &mockRawService{name: "testrawservice"}
	event := &mockEvent{kind: "testevent"}
	publisher := &mockService{name: "publisher"}

	m.Subscribe(service, "test/topic")
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)

	kinds, payloads := service.GetRawEvents()
	if len(kinds) != 1 {
		t.Errorf("expected 1 raw event, got %d", len(kinds))
	}

	if kinds[0] != "testevent" {
		t.Errorf("expected kind 'testevent', got '%s'", kinds[0])
	}

	if payloads[0] != event {
		t.Error("expected payload to match the event")
	}
}

func TestManagerPublishHierarchicalTopics(t *testing.T) {
	m := newManager()

	rootService := &mockService{name: "rootservice"}
	levelService := &mockService{name: "levelservice"}
	leafService := &mockService{name: "leafservice"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "hierarchicalevent"}

	m.Subscribe(rootService, "app")
	m.Subscribe(levelService, "app/module")
	m.Subscribe(leafService, "app/module/component")

	m.Publish(publisher, "app/module/component", event)

	time.Sleep(10 * time.Millisecond)

	rootEvents := rootService.GetEvents()
	levelEvents := levelService.GetEvents()
	leafEvents := leafService.GetEvents()

	if len(rootEvents) != 1 {
		t.Errorf("expected root service to receive 1 event, got %d", len(rootEvents))
	}

	if len(levelEvents) != 1 {
		t.Errorf("expected level service to receive 1 event, got %d", len(levelEvents))
	}

	if len(leafEvents) != 1 {
		t.Errorf("expected leaf service to receive 1 event, got %d", len(leafEvents))
	}
}

func TestManagerPublishNilEvent(t *testing.T) {
	m := newManager()
	service := &mockService{name: "testservice"}
	publisher := &mockService{name: "publisher"}

	m.Subscribe(service, "test/topic")
	m.Publish(publisher, "test/topic", nil)

	time.Sleep(10 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events for nil event, got %d", len(events))
	}
}

func TestManagerPublishNoSubscribers(t *testing.T) {
	m := newManager()
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	m.Publish(publisher, "nonexistent/topic", event)

	time.Sleep(10 * time.Millisecond)
}

func TestManagerConcurrentSubscribeAndPublish(t *testing.T) {
	m := newManager()
	event := &mockEvent{kind: "concurrentevent"}
	publisher := &mockService{name: "publisher"}

	var wg sync.WaitGroup
	services := make([]*mockService, 10)

	for i := 0; i < 10; i++ {
		services[i] = &mockService{name: "service" + string(rune('0'+i))}
		wg.Add(1)
		go func(svc *mockService) {
			defer wg.Done()
			m.Subscribe(svc, "concurrent/test")
		}(services[i])
	}

	wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Publish(publisher, "concurrent/test", event)
	}()

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	totalEvents := 0
	for _, service := range services {
		events := service.GetEvents()
		totalEvents += len(events)
	}

	if totalEvents != 10 {
		t.Errorf("expected 10 total events across all services, got %d", totalEvents)
	}
}

func TestManagerWithLogger(t *testing.T) {
	logger := log.Default
	m := New(WithLogger(logger))

	manager := m.(*manager)
	if manager.log == nil {
		t.Error("expected logger to be set")
	}
}

func TestManagerPublishDualHandler(t *testing.T) {
	m := newManager()
	service := &mockDualService{name: "dualservice"}
	event := &mockEvent{kind: "testevent"}
	publisher := &mockService{name: "publisher"}

	m.Subscribe(service, "test/topic")
	m.Publish(publisher, "test/topic", event)

	time.Sleep(10 * time.Millisecond)

	// Both handlers should be called
	events := service.GetEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event from EventHandler, got %d", len(events))
	}

	kinds, payloads := service.GetRawEvents()
	if len(kinds) != 1 {
		t.Errorf("expected 1 raw event from RawEventHandler, got %d", len(kinds))
	}

	if len(payloads) != 1 {
		t.Errorf("expected 1 payload from RawEventHandler, got %d", len(payloads))
	}

	if events[0].Kind() != "testevent" {
		t.Errorf("expected event kind 'testevent', got '%s'", events[0].Kind())
	}

	if kinds[0] != "testevent" {
		t.Errorf("expected raw event kind 'testevent', got '%s'", kinds[0])
	}
}
