package pubsub

import (
	"testing"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func TestMemoryBackendNew(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	if backend == nil {
		t.Fatal("expected backend to be created")
	}
}

func TestMemoryBackendSubscribeGetSubscribers(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	service := &mockService{name: "testservice"}

	err := backend.Subscribe(service, "test/topic")
	if err != nil {
		t.Fatalf("expected no error on subscribe, got %v", err)
	}

	subscribers := backend.GetSubscribers("test/topic")
	if len(subscribers) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(subscribers))
	}

	if len(subscribers) > 0 && subscribers[0].Name() != "testservice" {
		t.Errorf("expected subscriber name 'testservice', got '%s'", subscribers[0].Name())
	}
}

func TestMemoryBackendHierarchicalTopics(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	rootService := &mockService{name: "rootservice"}
	levelService := &mockService{name: "levelservice"}
	leafService := &mockService{name: "leafservice"}

	backend.Subscribe(rootService, "app")
	backend.Subscribe(levelService, "app/module")
	backend.Subscribe(leafService, "app/module/component")

	// Get subscribers for a deeply nested topic
	subscribers := backend.GetSubscribers("app/module/component")

	// All three services should be returned (hierarchical matching)
	if len(subscribers) != 3 {
		t.Errorf("expected 3 subscribers for hierarchical matching, got %d", len(subscribers))
	}

	// Verify all services are in the list
	names := make(map[string]bool)
	for _, sub := range subscribers {
		names[sub.Name()] = true
	}

	if !names["rootservice"] {
		t.Error("expected rootservice to be in subscribers")
	}
	if !names["levelservice"] {
		t.Error("expected levelservice to be in subscribers")
	}
	if !names["leafservice"] {
		t.Error("expected leafservice to be in subscribers")
	}
}

func TestMemoryBackendUnsubscribe(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	service := &mockService{name: "testservice"}

	backend.Subscribe(service, "test/topic")

	subscribers := backend.GetSubscribers("test/topic")
	if len(subscribers) != 1 {
		t.Errorf("expected 1 subscriber before unsubscribe, got %d", len(subscribers))
	}

	backend.Unsubscribe(service, "test/topic")

	subscribers = backend.GetSubscribers("test/topic")
	if len(subscribers) != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", len(subscribers))
	}
}

func TestMemoryBackendMultipleSubscribers(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	service1 := &mockService{name: "service1"}
	service2 := &mockService{name: "service2"}
	service3 := &mockService{name: "service3"}

	backend.Subscribe(service1, "test/topic")
	backend.Subscribe(service2, "test/topic")
	backend.Subscribe(service3, "test/topic")

	subscribers := backend.GetSubscribers("test/topic")
	if len(subscribers) != 3 {
		t.Errorf("expected 3 subscribers, got %d", len(subscribers))
	}

	// Unsubscribe middle service
	backend.Unsubscribe(service2, "test/topic")

	subscribers = backend.GetSubscribers("test/topic")
	if len(subscribers) != 2 {
		t.Errorf("expected 2 subscribers after unsubscribe, got %d", len(subscribers))
	}

	// Verify the right services remain
	names := make(map[string]bool)
	for _, sub := range subscribers {
		names[sub.Name()] = true
	}

	if !names["service1"] {
		t.Error("expected service1 to be in subscribers")
	}
	if names["service2"] {
		t.Error("expected service2 to NOT be in subscribers")
	}
	if !names["service3"] {
		t.Error("expected service3 to be in subscribers")
	}
}

func TestMemoryBackendClose(t *testing.T) {
	backend := NewMemoryBackend(log.Default)
	err := backend.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestMemoryBackendNilService(t *testing.T) {
	backend := NewMemoryBackend(log.Default)

	// Should not panic
	err := backend.Subscribe(nil, "test/topic")
	if err != nil {
		t.Errorf("expected no error for nil subscribe, got %v", err)
	}

	// Get subscribers should not include nil
	subscribers := backend.GetSubscribers("test/topic")
	if len(subscribers) != 0 {
		t.Errorf("expected 0 subscribers for nil service, got %d", len(subscribers))
	}
}

func TestMemoryBackendSetDispatcher(t *testing.T) {
	backend := NewMemoryBackend(log.Default)

	// SetDispatcher should not panic (it's a no-op for memory backend)
	backend.SetDispatcher(func(subscribers []common.Named, topic string, event common.Event) {
		// No-op
	})

	// Backend should still work after SetDispatcher
	service := &mockService{name: "testservice"}
	backend.Subscribe(service, "test/topic")

	subscribers := backend.GetSubscribers("test/topic")
	if len(subscribers) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(subscribers))
	}
}
