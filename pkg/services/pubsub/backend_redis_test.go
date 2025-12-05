package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// getTestRedisClient returns a Redis client for testing.
// Tests will be skipped if Redis is not available.
func getTestRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use a separate DB for testing
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping Redis tests: Redis not available at localhost:6379: %v", err)
	}

	// Clean up the test DB
	client.FlushDB(ctx)

	return client
}

func TestRedisBackendNew(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, err := NewRedisBackend(client, log.Default)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if backend == nil {
		t.Fatal("expected backend to be created")
	}

	backend.Close()
}

func TestRedisBackendNewWithNilClient(t *testing.T) {
	backend, err := NewRedisBackend(nil, log.Default)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if backend != nil {
		t.Error("expected nil backend for nil client")
	}
}

func TestRedisBackendSubscribePublish(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	service := &mockService{name: "testservice"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	err := backend.Subscribe(service, "test/topic")
	if err != nil {
		t.Fatalf("expected no error on subscribe, got %v", err)
	}

	// Give Redis time to process subscription
	time.Sleep(50 * time.Millisecond)

	err = backend.Publish(publisher, "test/topic", event)
	if err != nil {
		t.Fatalf("expected no error on publish, got %v", err)
	}

	// Give Redis time to deliver message
	time.Sleep(100 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestRedisBackendHierarchicalTopics(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	rootService := &mockService{name: "rootservice"}
	levelService := &mockService{name: "levelservice"}
	leafService := &mockService{name: "leafservice"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "hierarchicalevent"}

	backend.Subscribe(rootService, "app")
	backend.Subscribe(levelService, "app/module")
	backend.Subscribe(leafService, "app/module/component")

	// Give Redis time to process subscriptions
	time.Sleep(50 * time.Millisecond)

	backend.Publish(publisher, "app/module/component", event)

	// Give Redis time to deliver messages
	time.Sleep(100 * time.Millisecond)

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

func TestRedisBackendUnsubscribe(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	service := &mockService{name: "testservice"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	backend.Subscribe(service, "test/topic")
	time.Sleep(50 * time.Millisecond)

	backend.Unsubscribe(service, "test/topic")
	time.Sleep(50 * time.Millisecond)

	backend.Publish(publisher, "test/topic", event)
	time.Sleep(100 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events after unsubscribe, got %d", len(events))
	}
}

func TestRedisBackendMultipleSubscribers(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	service1 := &mockService{name: "service1"}
	service2 := &mockService{name: "service2"}
	service3 := &mockService{name: "service3"}
	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	backend.Subscribe(service1, "test/topic")
	backend.Subscribe(service2, "test/topic")
	backend.Subscribe(service3, "test/topic")

	time.Sleep(50 * time.Millisecond)

	// Unsubscribe middle service
	backend.Unsubscribe(service2, "test/topic")
	time.Sleep(50 * time.Millisecond)

	backend.Publish(publisher, "test/topic", event)
	time.Sleep(100 * time.Millisecond)

	events1 := service1.GetEvents()
	events2 := service2.GetEvents()
	events3 := service3.GetEvents()

	if len(events1) != 1 {
		t.Errorf("expected service1 to receive 1 event, got %d", len(events1))
	}

	if len(events2) != 0 {
		t.Errorf("expected service2 to receive 0 events (unsubscribed), got %d", len(events2))
	}

	if len(events3) != 1 {
		t.Errorf("expected service3 to receive 1 event, got %d", len(events3))
	}
}

func TestRedisBackendNilService(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	publisher := &mockService{name: "publisher"}
	event := &mockEvent{kind: "testevent"}

	// Should not panic
	err := backend.Subscribe(nil, "test/topic")
	if err != nil {
		t.Errorf("expected no error for nil subscribe, got %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = backend.Publish(publisher, "test/topic", event)
	if err != nil {
		t.Errorf("expected no error for publish, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestRedisBackendNilEvent(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)
	defer backend.Close()

	service := &mockService{name: "testservice"}
	publisher := &mockService{name: "publisher"}

	backend.Subscribe(service, "test/topic")
	time.Sleep(50 * time.Millisecond)

	err := backend.Publish(publisher, "test/topic", nil)
	if err != nil {
		t.Errorf("expected no error for nil event, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	events := service.GetEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events for nil event, got %d", len(events))
	}
}

func TestRedisBackendClose(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	backend, _ := NewRedisBackend(client, log.Default)

	err := backend.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}
