# Example Service

This directory contains an example service implementation demonstrating the Framingo service architecture and patterns.

## Overview

The `example` service is a minimal implementation showcasing how to create a service using the Framingo framework. It implements the core service interfaces and provides a simple "Hello World" functionality.

## Structure

```
example/
├── manager.go    # Service manager implementation
├── model.go      # Interface definitions
└── option.go     # Configuration options
```

## Files

### [manager.go](manager.go)

Contains the main service manager implementation (`manager` struct) and implements:
- `Manager` interface from [model.go](model.go)
- Service lifecycle methods: `Init()`, `Start()`, `Stop()`
- Core functionality: `HelloWorld()` method

The `HelloWorld` method demonstrates a simple business operation that saves data to the database:
```go
func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error) {
    m.log.Info("hello world!")

    // Create ORM model for database operation
    ormModel := &orm.Helloworld{
        Message: message,
    }

    // Save to database
    if err := m.db.FromContext(ctx).Create(ormModel).Error; err != nil {
        return nil, errors.Wrap(err)
    }

    // Convert ORM to entity for return
    result := &entity.Helloworld{
        ID:        ormModel.ID,
        Message:   ormModel.Message,
        CreatedAt: ormModel.CreatedAt,
        UpdatedAt: ormModel.UpdatedAt,
    }

    return result, nil
}
```

### [model.go](model.go)

Defines the `Manager` interface which extends:
- `common.Service` - Basic service interface
- `common.Initializable` - Initialization support
- `common.Debuggable` - Debug information support
- `common.Daemon` - Daemon lifecycle management

Custom methods:
- `HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error)` - Example business logic that accepts a message string and returns a Helloworld entity

### [option.go](option.go)

Provides functional options for service configuration:
- `WithLogger(logger log.Logger)` - Configure custom logger

**Note**: The database dependency is now a required parameter in the `New()` function rather than an optional configuration.

## Type Separation

The service uses different type structures for different purposes:

### Entity Type ([pkg/types/entity](../types/entity/example.go))
Used for business logic and API responses:
```go
type Helloworld struct {
    ID        int64     `json:"id"`
    Message   string    `json:"message"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### ORM Type ([pkg/types/orm](../types/orm/example.go))
Used for database operations:
```go
type Helloworld struct {
    ID        int64     `gorm:"primaryKey"`
    Message   string    `gorm:"type:text;not null"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
```

See [pkg/types/README.md](../types/README.md) for more details on type separation.

## Usage

### Creating a Service Instance

```go
import (
    "github.com/xhanio/framingo/example/pkg/services/example"
    "github.com/xhanio/framingo/pkg/services/db"
)

// Database is a required dependency
dbManager := db.New(/* db options */)

// Create with default logger
svc := example.New(dbManager)

// Create with custom logger
svc := example.New(
    dbManager,
    example.WithLogger(customLogger),
)
```

### Service Lifecycle

```go
// Initialize the service
if err := svc.Init(); err != nil {
    log.Fatal(err)
}

// Start the service
ctx := context.Background()
if err := svc.Start(ctx); err != nil {
    log.Fatal(err)
}

// Use the service
result, err := svc.HelloWorld(ctx, "Hello, Framingo!")
if err != nil {
    log.Error(err)
}
fmt.Printf("Message: %s\n", result.Message) // Output: Message: Hello, Framingo!

// Stop the service (with wait)
if err := svc.Stop(true); err != nil {
    log.Error(err)
}
```

## Key Features

- **Lifecycle Management**: Implements standard Init/Start/Stop pattern
- **Context Support**: Proper context handling for cancellation
- **Goroutine Safety**: Uses WaitGroup for graceful shutdown
- **Logging**: Integrated logging support with customizable logger
- **Service Discovery**: Automatic service name detection using reflection

## Implementation Details

### Service Name

The service name is automatically derived from the package path using reflection:
```go
func (m *manager) Name() string {
    if m.name == "" {
        m.name = path.Join(reflectutil.Locate(m))
    }
    return m.name
}
```

### Dependencies

The example service depends on the database manager:
```go
func (m *manager) Dependencies() []common.Service {
    return []common.Service{m.db}
}
```

The database dependency is injected through the constructor to ensure it's always available.

### Graceful Shutdown

The service properly handles shutdown by:
1. Listening for context cancellation
2. Using WaitGroup to track running goroutines
3. Providing optional blocking wait during `Stop()`

## Extending This Example

To create a new service based on this example:

1. Copy the `example` directory to a new service name
2. Update package name and interface methods in [model.go](model.go)
3. Implement business logic in [manager.go](manager.go)
4. Add configuration options in [option.go](option.go)
5. Add dependencies in `Dependencies()` method if needed
6. Implement actual work in the `Start()` goroutine

## See Also

- [Framingo Service Architecture](../../../pkg/types/common/)
- [Common Service Interfaces](../../../pkg/types/common/)
