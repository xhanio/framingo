# Pub/Sub Service

A publish/subscribe message bus for inter-service communication. Supports hierarchical topics and pluggable backends (memory, Redis, Kafka).

## Overview

The pubsub service allows services to communicate asynchronously via topics. It integrates with Framingo's service lifecycle — services that implement `common.MessageHandler` or `common.RawMessageHandler` receive messages automatically when subscribed.

Key characteristics:
- **Hierarchical topics**: subscribing to `"app"` receives messages from `"app"`, `"app/module"`, `"app/module/component"`, etc.
- **Self-delivery skip**: a publisher does not receive its own messages
- **Pluggable drivers**: memory (in-process), Redis (cross-instance), Kafka (cross-instance)
- **Typed and raw messages**: supports both `common.Message` (typed) and raw `(kind, payload)` dispatch

## Structure

```
pubsub/
├── model.go          # Manager interface
├── manager.go        # Implementation
├── option.go         # Functional options
├── manager_test.go   # Tests
└── driver/
    ├── model.go      # Driver interface
    ├── util.go       # Shared types (subscriber, topicMatches)
    ├── memory.go     # In-process driver (trie-based)
    ├── redis.go      # Redis driver (cross-instance via Redis Pub/Sub)
    ├── kafka.go      # Kafka driver (cross-instance via Kafka consumer groups)
    ├── memory_test.go
    ├── redis_test.go
    └── kafka_test.go
```

## Usage

### Creating the Bus

```go
import (
    "github.com/xhanio/framingo/pkg/services/pubsub"
    "github.com/xhanio/framingo/pkg/services/pubsub/driver"
)

// In-memory driver (single-instance)
bus := pubsub.New(
    driver.NewMemory(logger),
    pubsub.WithLogger(logger),
)
```

### Subscribing

Subscribe a named service to a topic. The service must implement `common.MessageHandler` and/or `common.RawMessageHandler` to receive messages.

```go
bus.Subscribe(myService, "/events")
```

Hierarchical matching: subscribing to `"/events"` also receives messages published to `"/events/user"`, `"/events/user/created"`, etc.

### Publishing

```go
// Publish with explicit topic and kind
bus.Publish(sender, "/events/user", "user.created", payload)

// SendMessage: topic is derived from sender.Name(), kind from message.Kind()
bus.SendMessage(ctx, sender, myMessage)

// SendRawMessage: topic is derived from sender.Name()
bus.SendRawMessage(ctx, sender, "custom.kind", rawPayload)
```

### Unsubscribing

```go
bus.Unsubscribe(myService, "/events")
```

## Message Handlers

Services receive messages by implementing these interfaces from `pkg/types/common`:

```go
// For typed messages (payload implements common.Message)
type MessageHandler interface {
    HandleMessage(ctx context.Context, e Message) error
}

// For any payload
type RawMessageHandler interface {
    HandleRawMessage(ctx context.Context, kind string, payload any) error
}
```

If a payload implements `common.Message`, both handlers are called. Otherwise, only `RawMessageHandler` is called.

## Drivers

### Memory

In-process driver using a trie for topic matching. No external dependencies.

```go
d := driver.NewMemory(logger)
```

### Redis

Cross-instance driver using Redis Pub/Sub pattern subscriptions. Messages are delivered locally and published to Redis for other instances.

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
d, err := driver.NewRedis(client, logger)
```

### Kafka

Cross-instance driver using Kafka. Each instance gets a unique consumer group for broadcast semantics (every instance receives all messages).

```go
d, err := driver.NewKafka(
    []string{"localhost:9092"}, // brokers
    "my-app",                   // group ID prefix
    logger,
)
```

## Manager Interface

```go
type Manager interface {
    common.Service
    common.Daemon
    common.Initializable
    common.Debuggable
    common.MessageSender
    common.RawMessageSender

    Publish(from common.Named, topic string, kind string, payload any)
    Subscribe(svc common.Named, topic string)
    Unsubscribe(svc common.Named, topic string)
}
```

## Options

| Option | Description |
|--------|-------------|
| `WithLogger(log.Logger)` | Set the logger |
| `WithName(string)` | Override the auto-derived name |

## Topic Hierarchy Example

In the example server component, all services are subscribed to three levels:

```go
bus.Subscribe(svc, "/")
bus.Subscribe(svc, "/components/{component}")
bus.Subscribe(svc, "/components/{component}/services/{service}")
```

This allows publishing at different scopes:
- `/` — broadcast to all services across all components
- `/components/myapp` — target all services in a specific component
- `/components/myapp/services/db` — target a specific service

## See Also

- [Common Message Interfaces](../../types/common/message.go)
- [Example Server Component](../../../example/pkg/components/server/example/)
- [Service Controller](../app/)
