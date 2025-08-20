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

func TestManagerNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("expected manager to be created")
	}

	if m.Name() == "" {
		t.Error("expected manager to have a name")
	}

	deps := m.Dependencies()
	if deps != nil {
		t.Error("expected no dependencies")
	}
}

func TestManagerSubscribe(t *testing.T) {
	m := newManager()
	service := &mockService{name: "testservice"}

	m.Subscribe(service, "test/topic")

	if node, ok := m.topics.Find("test/topic"); !ok {
		t.Error("expected topic to be found in trie")
	} else if subscribers := node.Value(); len(subscribers) != 1 {
		t.Error("expected one subscriber")
	} else if subscribers[0] != service {
		t.Error("expected service to be subscribed")
	}
}

func TestManagerSubscribeMultipleServices(t *testing.T) {
	m := newManager()
	service1 := &mockService{name: "service1"}
	service2 := &mockService{name: "service2"}

	m.Subscribe(service1, "test/topic")
	m.Subscribe(service2, "test/topic")

	node, ok := m.topics.Find("test/topic")
	if !ok {
		t.Fatal("expected topic to be found in trie")
	}

	subscribers := node.Value()

	if len(subscribers) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(subscribers))
	}
}

func TestManagerSubscribeNilService(t *testing.T) {
	m := newManager()

	m.Subscribe(nil, "test/topic")

	if node, ok := m.topics.Find("test/topic"); ok {
		t.Error("expected no topic to be created for nil service")
	} else if node != nil {
		t.Error("expected node to be nil")
	}
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
	if manager.log != logger {
		t.Error("expected logger to be set")
	}
}

func TestManagerWithName(t *testing.T) {
	name := "custommessenger"
	m := New(WithName(name))

	if m.Name() != name {
		t.Errorf("expected name '%s', got '%s'", name, m.Name())
	}
}
