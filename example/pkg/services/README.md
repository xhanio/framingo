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

The `HelloWorld` method demonstrates a simple business operation:
```go
func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error) {
    m.log.Info("hello world!")
    result := &entity.Helloworld{
        Message: message,
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

## Entity Types

The service uses entity types from [pkg/types/entity](../types/entity/example.go):

### Helloworld Entity
```go
type Helloworld struct {
    Message string `json:"message"`
}
```

This entity is returned by the `HelloWorld` method and can be used for API responses or further processing.

## Usage

### Creating a Service Instance

```go
import "github.com/xhanio/framingo/example/pkg/services/example"

// Create with default settings
svc := example.New()

// Create with custom logger
svc := example.New(
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

The example service has no dependencies:
```go
func (m *manager) Dependencies() []common.Service {
    return []common.Service{}
}
```

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
