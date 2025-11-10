# Example Middleware

This directory contains an example middleware implementation demonstrating the Framingo middleware architecture and request processing patterns.

## Overview

The `example` middleware showcases how to create HTTP middleware using the Framingo framework. This particular implementation demonstrates request body decompression for deflate-encoded requests, providing a practical example of request preprocessing.

## Structure

```
example/
└── middleware.go    # Middleware implementation
```

## Files

### [middleware.go](middleware.go)

Contains the middleware implementation with:
- `middleware` struct that implements `api.Middleware` interface
- Automatic middleware name detection using reflection
- Request body decompression logic for deflate-encoded content

Key methods:
- `Name()` - Returns the middleware name (automatically derived from package name)
- `Dependencies()` - Lists required service dependencies (none for this example)
- `Func(next echo.HandlerFunc) echo.HandlerFunc` - Core middleware logic

## Usage

### Creating a Middleware

```go
import "github.com/xhanio/framingo/example/pkg/middlewares/example"

// Create middleware instance
mw := example.New()
```

### Registering with API Server

```go
import (
    "github.com/xhanio/framingo/pkg/services/api/server"
    "github.com/xhanio/framingo/example/pkg/middlewares/example"
)

// Create server manager
serverMgr := server.New()

// Register middleware
err := serverMgr.RegisterMiddlewares(example.New())
if err != nil {
    log.Fatal(err)
}
```

### Using in Router Configuration

Reference the middleware in your router's [router.yaml](../routers/example/router.yaml):

```yaml
handlers:
  - method: GET
    path: /example
    func: Example
    middleware: example  # Middleware name
```

## Middleware Implementation

The example middleware handles deflate compression:

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        r := c.Request()
        if r.Header.Get("Content-Encoding") == "deflate" {
            reader, err := zlib.NewReader(r.Body)
            if err != nil {
                return errors.BadRequest.Newf("failed to deflate request body: %s", err)
            }
            // Replace request body with deflated stream
            c.Request().Body = reader
        }
        return next(c)
    }
}
```

### How It Works

1. **Check Request Headers**: Inspects `Content-Encoding` header
2. **Decompress if Needed**: Creates zlib reader for deflate-encoded bodies
3. **Replace Request Body**: Swaps compressed body with decompressed stream
4. **Continue Chain**: Calls `next(c)` to proceed to next middleware/handler
5. **Error Handling**: Returns `BadRequest` error if decompression fails

## Middleware Interface

The `api.Middleware` interface requires:

```go
type Middleware interface {
    common.Service           // Name() and Dependencies()
    Func(echo.HandlerFunc) echo.HandlerFunc  // Middleware logic
}
```

All middlewares must implement:
- `Name()` - Unique identifier for the middleware
- `Dependencies()` - List of required services
- `Func()` - Middleware wrapping function

## Built-in Server Middlewares

The API server ([pkg/services/api/server](../../../pkg/services/api/server/middleware.go)) provides several built-in middlewares:

### Error Middleware
Wraps and handles errors from handlers:
- Converts errors to API error responses
- Stores error in context for logging
- Standardizes error format

### Info Middleware
Extracts request information:
- Parses request details (IP, path, trace ID)
- Injects request info into context
- Captures response timing and status

### Logger Middleware
Logs request/response information:
- Accesses request info from context
- Prints formatted log entries
- Supports polling API suppression

### Recover Middleware
Recovers from panics:
- Catches panics in handler chain
- Logs stack traces
- Converts panics to proper errors

### Throttle Middleware
Implements rate limiting:
- Per-IP and per-path rate limiting
- Configurable per-handler or server-wide
- Uses token bucket algorithm
- Returns `TooManyRequests` when limit exceeded

## Example Request Flow

When a request hits an endpoint with the example middleware:

```bash
curl -X POST http://localhost:8080/demo/example \
  -H "Content-Encoding: deflate" \
  --data-binary @compressed.bin
```

**Processing Order**:
1. Server middleware (Recover, Info, Logger, etc.)
2. **Example middleware** (decompress body)
3. Handler executes with decompressed body
4. Response flows back through middleware chain

## Creating Custom Middlewares

### 1. Define the Middleware Struct

```go
package mymiddleware

import (
    "github.com/labstack/echo/v4"
    "github.com/xhanio/framingo/pkg/types/api"
    "github.com/xhanio/framingo/pkg/types/common"
)

type middleware struct {
    // Add dependencies or configuration here
}

func New() api.Middleware {
    return &middleware{}
}
```

### 2. Implement Required Methods

```go
func (m *middleware) Name() string {
    return "mymiddleware"
}

func (m *middleware) Dependencies() []common.Service {
    return nil // or list required services
}
```

### 3. Implement Middleware Logic

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Pre-processing logic here

        // Call next middleware/handler
        err := next(c)

        // Post-processing logic here

        return err
    }
}
```

### 4. Register and Use

```go
// Register with server
serverMgr.RegisterMiddlewares(mymiddleware.New())

// Reference in router.yaml
handlers:
  - method: GET
    path: /api/endpoint
    func: Handler
    middleware: mymiddleware
```

## Common Middleware Patterns

### Authentication
```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        token := c.Request().Header.Get("Authorization")
        if !validateToken(token) {
            return errors.Unauthorized.New("invalid token")
        }
        c.Set("user", getUserFromToken(token))
        return next(c)
    }
}
```

### Request Validation
```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        if c.Request().ContentLength > maxSize {
            return errors.BadRequest.New("request too large")
        }
        return next(c)
    }
}
```

### Response Modification
```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        err := next(c)
        c.Response().Header().Set("X-Custom-Header", "value")
        return err
    }
}
```

## Key Features

- **Automatic Naming**: Uses reflection for middleware identification
- **Dependency Injection**: Supports service dependencies
- **Echo Integration**: Built on Echo middleware pattern
- **Error Handling**: Proper error propagation and handling
- **Composability**: Can be chained with other middlewares

## Best Practices

1. **Naming**: Use descriptive, unique middleware names
2. **Dependencies**: Declare all service dependencies explicitly
3. **Error Handling**: Return proper error types for different scenarios
4. **Performance**: Keep middleware logic lightweight
5. **Order**: Consider middleware execution order (defined in server configuration)
6. **Context**: Use `c.Set()` to share data between middlewares and handlers
7. **Next Call**: Always call `next(c)` unless you want to short-circuit the chain

## Middleware Execution Order

Middlewares execute in the order they're registered on the server:

```
Request  Recover  Info  Throttle  Logger  Custom (example)  Handler
                                                             Response
```

Each middleware can:
- Modify the request before passing to next
- Short-circuit the chain by returning without calling next
- Modify the response after next returns

## See Also

- [API Server Middleware](../../../pkg/services/api/server/middleware.go)
- [API Server Manager](../../../pkg/services/api/server/manager.go)
- [API Types](../../../pkg/types/api/model.go)
- [Example Router](../routers/example/)
- [Echo Middleware Guide](https://echo.labstack.com/middleware/)
