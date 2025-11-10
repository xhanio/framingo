# Example Server Component

This directory contains a complete server component implementation demonstrating how to build a production-ready HTTP API server using the Framingo framework.

## Overview

The `example` server component is a comprehensive implementation that orchestrates all parts of a Framingo application, including:
- Configuration management via Viper
- Database connectivity and migration
- Service lifecycle management
- HTTP API server setup with routers and middlewares
- Signal handling for graceful shutdown
- Profiling support (pprof)

This component serves as the main entry point for your application and demonstrates best practices for assembling a complete server.

## Structure

```
example/
├── manager.go    # Main server implementation and initialization
├── model.go      # Server interface and configuration
└── api.go        # API router and middleware registration
```

## Files

### [model.go](model.go)

Defines the server interface and configuration:

```go
type Config struct {
    Path string  // Configuration file path
}

type Server interface {
    common.Daemon         // Start() and Stop()
    common.Initializable  // Init()
    common.Debuggable     // Info()
}
```

### [manager.go](manager.go)

Contains the main server implementation with:
- Configuration loading via Viper
- Logger initialization with file rotation
- Database manager setup
- Service controller initialization
- API server configuration from YAML
- Service dependency resolution via topological sort
- Signal handling (SIGINT, SIGUSR1, SIGUSR2)
- pprof profiling support

### [api.go](api.go)

Handles API-specific initialization:
- Middleware registration
- Router registration
- Integration with [pkg/middlewares](../../middlewares/example/)
- Integration with [pkg/routers](../../routers/example/)

## Usage

### Creating and Starting a Server

```go
package main

import (
    "context"
    "log"

    "github.com/xhanio/framingo/example/pkg/components/server/example"
)

func main() {
    // Create server with config file path
    srv := example.New(example.Config{
        Path: "/path/to/config.yaml",
    })

    // Initialize the server
    if err := srv.Init(); err != nil {
        log.Fatal(err)
    }

    // Start the server (blocks until SIGINT)
    if err := srv.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Configuration File Structure

The server expects a YAML configuration file with the following structure:

```yaml
# Logging configuration
log:
  level: 0  # 0=Debug, 1=Info, 2=Warn, 3=Error
  file: /var/log/app.log
  rotation:
    max_size: 100      # MB
    max_backups: 3
    max_age: 7         # days

# Database configuration
db:
  type: postgres
  source:
    host: 127.0.0.1
    port: 5432
    user: dbuser
    password: dbpass
    dbname: mydb
  migration:
    dir: ./migrations
    version: 1
  connection:
    max_open: 10
    max_idle: 5
    max_lifetime: 1h
    exec_timeout: 30s

# API server configuration
api:
  http:  # Server name
    host: 0.0.0.0
    port: 8080
    prefix: /api/v1
    throttle:
      rps: 100.0
      burst_size: 200
    # Optional TLS
    # cert: /path/to/cert.pem
    # key: /path/to/key.pem

# Optional: pprof profiling
pprof:
  port: 6060
```

## Server Lifecycle

### Initialization Flow

The `Init()` method performs the following steps:

1. **Load Configuration**
   - Read YAML config file via Viper
   - Support environment variable overrides

2. **Initialize Logger**
   - Configure log level
   - Set up file rotation

3. **Initialize Database Manager**
   - Configure connection pool
   - Set up migrations

4. **Initialize Service Controller**
   - Create service manager
   - Handle service dependencies

5. **Initialize Business Services**
   - Create example service (see [pkg/services](../../services/example/))

6. **Initialize API Server**
   - Configure multiple API servers from config
   - Set up throttling and TLS if configured

7. **Register Services**
   - Register all services with controller
   - Perform topological sort for dependency resolution

8. **Initialize API Components** (via [api.go](api.go:10))
   - Register middlewares (see [pkg/middlewares](../../middlewares/example/))
   - Register routers (see [pkg/routers](../../routers/example/))

9. **Pre/Post Initialization Hooks**
   - Call `Init()` on all services

### Startup Flow

The `Start()` method:

1. **Start All Services**
   - Launches services in dependency order
   - Runs in goroutine with panic recovery

2. **Enable pprof (if configured)**
   - Starts pprof HTTP server on configured port

3. **Set Up Signal Handlers**
   - SIGINT: Graceful shutdown
   - SIGUSR1: Print service info
   - SIGUSR2: Print stack traces

4. **Block Until Signal**
   - Waits for signals
   - Handles shutdown gracefully

### Shutdown Flow

The `Stop()` method:

1. Stops all services in reverse dependency order
2. Waits for all goroutines to complete
3. Closes database connections
4. Cleans up resources

## Service Dependencies

The server component manages several layers of services:

### Utility Services
- **Logger**: Application-wide logging
- **Database Manager**: Connection pool and migrations

### Business Services
- **Example Service**: Custom business logic (see [pkg/services/example](../../services/example/))

### API Services
- **Server Manager**: HTTP API server ([pkg/services/api/server](../../../../pkg/services/api/server/))
- **Routers**: HTTP route handlers (see [pkg/routers/example](../../routers/example/))
- **Middlewares**: Request processing pipeline (see [pkg/middlewares/example](../../middlewares/example/))

### Service Controller
- **Controller Manager**: Orchestrates service lifecycle ([pkg/services/controller](../../../pkg/services/controller/))

The controller automatically resolves dependencies using topological sorting:

```go
// Register services
m.services.Register(
    m.db,
    m.example,
)

// Automatically sort by dependencies
if err := m.services.TopoSort(); err != nil {
    return errors.Wrap(err)
}

// Add API server last (depends on all other services)
m.services.Register(m.api)
```

## API Initialization

The [api.go](api.go) file demonstrates how to wire up API components:

```go
func (m *manager) initAPI() error {
    // Register middlewares
    middlewares := []api.Middleware{
        mwexample.New(),  // pkg/middlewares/example
    }
    m.api.RegisterMiddlewares(middlewares...)

    // Register routers
    routers := []api.Router{
        example.New(m.example, m.log),  // pkg/routers/example
    }
    err := m.api.RegisterRouters(routers...)
    if err != nil {
        return errors.Wrap(err)
    }

    return nil
}
```

This connects:
- Middlewares from [pkg/middlewares/example](../../middlewares/example/)
- Routers from [pkg/routers/example](../../routers/example/)
- Services from [pkg/services/example](../../services/example/)

## Signal Handling

The server responds to Unix signals:

### SIGINT (Ctrl+C)
Triggers graceful shutdown:
```bash
kill -INT <pid>
# or press Ctrl+C
```

### SIGUSR1
Prints service information to stdout:
```bash
kill -USR1 <pid>
```

Example output:
```
Service: example/pkg/services/example
Status: running
Uptime: 1h23m45s
...
```

### SIGUSR2
Prints all goroutine stack traces:
```bash
kill -USR2 <pid>
```

Useful for debugging deadlocks or stuck goroutines.

## Profiling with pprof

If configured, the server starts a pprof HTTP server:

```yaml
pprof:
  port: 6060
```

Access profiling endpoints:
```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Web interface
open http://localhost:6060/debug/pprof/
```

## Environment Variables

Configuration values can be overridden via environment variables with the product name prefix:

```bash
export FRAMINGO_DB_HOST=postgres.example.com
export FRAMINGO_DB_PORT=5432
export FRAMINGO_API_HTTP_PORT=9090
```

The prefix is automatically determined from the product name.

## Configuration Priority

Configuration values are resolved in this order (highest to lowest priority):

1. Environment variables (with product prefix)
2. YAML configuration file
3. Default values in code

Example:
```go
Host: sliceutil.First(
    viper.GetString("db.source.host"),  // YAML config
    viper.GetString("DB_HOST"),         // Env var
    "127.0.0.1",                        // Default
)
```

## Multiple API Servers

The server supports multiple API servers running simultaneously:

```yaml
api:
  http:
    host: 0.0.0.0
    port: 8080
    prefix: /api/v1
  admin:
    host: 127.0.0.1  # Admin only on localhost
    port: 8081
    prefix: /admin
    cert: /path/to/admin-cert.pem
    key: /path/to/admin-key.pem
```

Routers can target specific servers via their YAML config:
```yaml
server: http  # or 'admin'
```

## TLS Configuration

Enable HTTPS by specifying cert and key paths:

```yaml
api:
  https:
    host: 0.0.0.0
    port: 8443
    prefix: /api/v1
    cert: /path/to/server-cert.pem
    key: /path/to/server-key.pem

# Optional: CA cert for client verification
ca:
  cert: /path/to/ca-cert.pem
```

## Key Features

- **Configuration Management**: Viper-based with YAML and env var support
- **Service Orchestration**: Automatic dependency resolution
- **Graceful Shutdown**: Proper cleanup on SIGINT
- **Database Integration**: Built-in migration and connection pooling
- **API Flexibility**: Support for multiple servers, TLS, throttling
- **Debugging Tools**: Signal-based introspection and pprof profiling
- **Logging**: Structured logging with file rotation
- **Production Ready**: Panic recovery and error handling

## Best Practices

1. **Configuration**: Use environment variables for secrets, YAML for structure
2. **Dependencies**: Declare all service dependencies explicitly
3. **Error Handling**: Always check and log initialization errors
4. **Graceful Shutdown**: Test SIGINT handling to ensure clean shutdown
5. **Monitoring**: Use pprof in development, integrate proper monitoring in production
6. **Logging**: Set appropriate log levels (Debug in dev, Info+ in production)
7. **Database**: Configure connection pools based on load

## Extending the Server

### Adding a New Service

1. **Create the service** (see [pkg/services](../../services/example/))
2. **Add to manager struct**:
   ```go
   type manager struct {
       // ...
       myService myservice.Manager
   }
   ```
3. **Initialize in Init()**:
   ```go
   m.myService = myservice.New(
       myservice.WithLogger(m.log),
   )
   ```
4. **Register with controller**:
   ```go
   m.services.Register(m.myService)
   ```

### Adding a New Router

1. **Create the router** (see [pkg/routers](../../routers/example/))
2. **Register in [api.go](api.go)**:
   ```go
   routers := []api.Router{
       example.New(m.example, m.log),
       myrouter.New(m.myService, m.log),
   }
   ```

### Adding a New Middleware

1. **Create the middleware** (see [pkg/middlewares](../../middlewares/example/))
2. **Register in [api.go](api.go)**:
   ```go
   middlewares := []api.Middleware{
       mwexample.New(),
       mymiddleware.New(),
   }
   ```

## See Also

- [Example Services](../../services/example/)
- [Example Routers](../../routers/example/)
- [Example Middlewares](../../middlewares/example/)
- [API Server Manager](../../../../pkg/services/api/server/)
- [Service Controller](../../../../pkg/services/controller/)
- [Database Manager](../../../../pkg/services/db/)
- [Viper Configuration](https://github.com/spf13/viper)
