# Example Router

This directory contains an example router implementation demonstrating the Framingo API router architecture and patterns.

## Overview

The `example` router showcases how to create HTTP API endpoints using the Framingo framework. It integrates with the example service and demonstrates routing configuration, handler implementation, and automatic handler discovery through reflection.

## Structure

```
example/
├── router.go       # Router implementation and configuration
├── handler.go      # HTTP request handlers
└── router.yaml     # Route definitions and mappings
```

## Files

### [router.go](router.go)

Contains the router implementation with:
- `router` struct that implements `api.Router` interface
- Service dependencies (example manager)
- Embedded YAML configuration via `//go:embed`
- Automatic router name detection using reflection

Key methods:
- `Name()` - Returns the router name
- `Dependencies()` - Lists required services
- `Config()` - Returns embedded YAML configuration

### [handler.go](handler.go)

Implements HTTP request handlers and handler discovery:
- `Example()` - Sample HTTP handler that returns "Good"
- `Handlers()` - Automatic handler discovery using reflection

The `Handlers()` method uses reflection to automatically discover all methods with the signature `func(echo.Context) error` and registers them as handlers.

### [router.yaml](router.yaml)

YAML configuration defining routes:
```yaml
server: http           # Server name to mount on
prefix: /demo          # Route group prefix
handlers:
  - method: GET        # HTTP method
    path: /example     # Route path
    func: Example      # Handler function name
    middleware: example # Middleware to apply
```

## Usage

### Creating a Router

```go
import (
    "github.com/xhanio/framingo/example/pkg/routers/example"
    exampleSvc "github.com/xhanio/framingo/example/pkg/services/example"
)

// Create service instance
svc := exampleSvc.New()

// Create router with service dependency
router := example.New(svc, logger)
```

### Router Configuration

The router is configured via the embedded [router.yaml](router.yaml):
- **server**: Name of the API server to mount on
- **prefix**: Base path for all routes in this router
- **handlers**: List of route definitions

### Route Definition

Each handler in the YAML config includes:
- `method` - HTTP method (GET, POST, PUT, DELETE, etc.)
- `path` - Route path (combined with prefix)
- `func` - Handler function name (must match method in [handler.go](handler.go))
- `middleware` - Optional middleware name

### Adding New Handlers

1. **Define the handler method in [handler.go](handler.go)**:
```go
func (r *router) MyNewHandler(c echo.Context) error {
    // Your logic here
    return c.JSON(200, map[string]string{
        "message": "Success",
    })
}
```

2. **Add route configuration in [router.yaml](router.yaml)**:
```yaml
handlers:
  - method: POST
    path: /my-endpoint
    func: MyNewHandler
    middleware: auth  # Optional
```

The handler will be automatically discovered by the `Handlers()` method through reflection.

## Example Endpoint

### Request

```bash
curl http://localhost:8080/demo/example
```

### Response

```
Good
```

The full URL is constructed as:
- Server base URL: `http://localhost:8080`
- Router prefix: `/demo`
- Handler path: `/example`
- Result: `http://localhost:8080/demo/example`

## Handler Discovery

The router uses reflection to automatically discover handlers:

```go
func (r *router) Handlers() map[string]echo.HandlerFunc {
    handlers := make(map[string]echo.HandlerFunc)
    rv := reflect.ValueOf(r)
    rt := reflect.TypeOf(r)

    for i := 0; i < rt.NumMethod(); i++ {
        method := rt.Method(i)
        if method.Name == "Handlers" {
            continue
        }
        methodValue := rv.Method(i)
        if handlerFunc, ok := methodValue.Interface().(func(echo.Context) error); ok {
            handlers[method.Name] = handlerFunc
        }
    }
    return handlers
}
```

This approach:
- Scans all methods on the router struct
- Filters methods matching the Echo handler signature
- Maps function names to handler functions
- Enables YAML-based handler references

## Dependencies

The example router depends on:
- `example.Manager` service ([pkg/services/example/](../services/example/))
- Echo web framework for HTTP handling

Dependencies are declared in the `Dependencies()` method and used for service orchestration.

## Key Features

- **YAML Configuration**: Declarative route definitions
- **Automatic Handler Discovery**: Reflection-based handler registration
- **Service Integration**: Clean dependency injection
- **Middleware Support**: Per-route middleware configuration
- **Echo Framework**: Built on the popular Echo web framework

## Best Practices

1. **Handler Naming**: Use descriptive names for handler methods
2. **Error Handling**: Always return proper HTTP status codes
3. **Service Usage**: Access business logic through injected services
4. **Path Organization**: Use meaningful prefixes and paths
5. **Middleware**: Apply appropriate middleware for authentication, logging, etc.

## See Also

- [Framingo API Types](../../../pkg/types/api/)
- [API Server Implementation](../../../pkg/services/api/server/)
- [Example Service](../services/example/)
- [Echo Framework Documentation](https://echo.labstack.com/)
