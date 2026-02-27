# Example Server Component

This directory contains a complete server component implementation demonstrating how to build a production-ready HTTP API server using the Framingo framework.

## Overview

The `example` server component is a comprehensive implementation that orchestrates all parts of a Framingo application, including:
- Instance-based Viper configuration with hot-reload support
- Database connectivity and migration
- Pub/sub message bus for inter-service communication
- Service lifecycle management with context-based config propagation
- HTTP API server setup with routers and middlewares
- Built-in signal handling (SIGINT, SIGTERM, SIGUSR1, SIGUSR2)
- Health monitoring with liveness/readiness probes and auto-restart
- Profiling support (pprof)

This component serves as the main entry point for your application and demonstrates best practices for assembling a complete server.

## Structure

```
example/
├── manager.go    # Server lifecycle (Init, Start, Stop) and service wiring
├── model.go      # Server interface
├── config.go     # Viper configuration setup and loading
├── service.go    # Service instance creation (logger, db, app manager, API)
├── signal.go     # OS signal handling (SIGINT, SIGTERM, SIGUSR1, SIGUSR2)
└── api.go        # API router and middleware registration
```

## Files

### [model.go](model.go)

Defines the server interface:

```go
type Server interface {
    common.Named          // Name()
    common.Daemon         // Start() and Stop()
    common.Initializable  // Init(ctx context.Context)
    common.Debuggable     // Info()
}
```

### [config.go](config.go)

Viper configuration setup and loading:
- Creates instance-based Viper (not global singleton)
- Sets environment variable prefix and enables `AutomaticEnv()`
- Reads YAML config file and enables hot-reload with `WatchConfig()`

### [service.go](service.go)

Service instance creation:
- Logger initialization with file rotation
- Database manager setup with connection pooling and migration
- Service controller (`app.Manager`) creation
- Pub/sub bus creation with configurable driver (memory, Redis, Kafka)
- Business service creation (e.g. example service)
- API server manager creation with per-server endpoint, throttle, and TLS configuration

### [manager.go](manager.go)

Server lifecycle orchestration:
- Service registration and dependency resolution via topological sort
- Pub/sub bus subscriptions for all services on hierarchical topics
- Service initialization and post-initialization (API wiring)
- Startup flow with pprof and signal handling
- Graceful shutdown

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
    srv := example.New("/path/to/config.yaml")

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

The `Init(ctx)` method performs the following steps:

1. **Load Configuration** (via [config.go](config.go))
   - Read YAML config file via instance-based Viper (not global singleton)
   - Support environment variable overrides
   - Enable config watching with `WatchConfig()`

2. **Create Service Instances** (via [service.go](service.go))
   - Initialize logger with file rotation
   - Create database manager with connection pooling and migration
   - Create service controller: `app.New(m.config, ...)`
   - Create pub/sub bus with driver (memory by default)
   - Create business services (e.g. example service with database dependency)
   - Create API server manager with per-server endpoint, throttle, and TLS configuration

3. **Register and Wire Services** (via [manager.go](manager.go))
   - Register all services with controller
   - Perform topological sort for dependency resolution
   - Register API server last (after topo sort) to ensure latest start
   - Subscribe all services to the bus on hierarchical topics (`/`, `/components/{component}`, `/components/{component}/services/{service}`)

4. **Initialize All Services**
   - Call `Init(ctx)` on all services (config is propagated via context using `confutil`)

5. **Initialize API Components** (via [api.go](api.go))
   - Register middlewares (see [pkg/middlewares](../../middlewares/example/))
   - Register routers (see [pkg/routers](../../routers/example/))

### Startup Flow

The `Start()` method:

1. **Start All Services**
   - Launches services in dependency order via the app manager

2. **Enable pprof (if configured)**
   - Starts pprof HTTP server on configured port

3. **Signal Handling**
   - SIGINT/SIGTERM: Graceful shutdown
   - SIGUSR1: Print service info to stdout
   - SIGUSR2: Print goroutine stack traces

4. **Health Monitoring (if configured)**
   - Periodic liveness/readiness checks on all services
   - Automatic restart on liveness failure with configurable retry policy

5. **Block Until Shutdown**
   - Waits for SIGINT/SIGTERM
   - Handles shutdown gracefully with configurable timeout

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

### System Services
- **Pub/Sub Bus**: Inter-service message bus ([pkg/services/pubsub](../../../../pkg/services/pubsub/))

### Business Services
- **Example Service**: Custom business logic (see [pkg/services/example](../../services/example/))

### API Services
- **Server Manager**: HTTP API server ([pkg/services/api/server](../../../../pkg/services/api/server/))
- **Routers**: HTTP route handlers (see [pkg/routers/example](../../routers/example/))
- **Middlewares**: Request processing pipeline (see [pkg/middlewares/example](../../middlewares/example/))

### Service Controller
- **Controller Manager**: Orchestrates service lifecycle ([pkg/services/app](../../../pkg/services/app/))

The controller automatically resolves dependencies using topological sorting and manages health monitoring:

```go
// Create service controller with config for propagation to services
m.services = app.New(m.config,
    app.WithLogger(m.log),
    app.WithShutdownTimeout(30*time.Second),     // Graceful shutdown timeout
    app.WithMonitorInterval(10*time.Second),      // Health check interval
    app.WithRestartPolicy(3),                     // Max restart attempts
    app.WithRestartDelay(5*time.Second),          // Delay between restarts
)

// Register services
m.services.Register(
    m.db,
    m.bus,
    m.example,
)

// Automatically sort by dependencies
if err := m.services.TopoSort(); err != nil {
    return errors.Wrap(err)
}

// Add API server last (depends on all other services)
m.services.Register(m.api)

// Subscribe all services to the bus on hierarchical topics
for _, svc := range m.services.Services() {
    m.bus.Subscribe(svc, "/")
    m.bus.Subscribe(svc, fmt.Sprintf("/components/%s", m.Name()))
    m.bus.Subscribe(svc, fmt.Sprintf("/components/%s/services/%s", m.Name(), svc.Name()))
}
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

Signal handling is managed by the server component in [signal.go](signal.go).

### Signal Handlers

| Signal | Action |
|--------|--------|
| `SIGINT` / `SIGTERM` | Graceful shutdown (`services.Stop(true)`) |
| `SIGUSR1` | Print service info (name, state, alive/ready, uptime) |
| `SIGUSR2` | Print all goroutine stack traces |

### Usage

```bash
# Graceful shutdown
kill -INT <pid>   # or press Ctrl+C

# Print service info (includes alive/ready/uptime)
kill -USR1 <pid>

# Print goroutine stack traces
kill -USR2 <pid>
```

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

The server uses **instance-based Viper** (not the global singleton). Configuration is:
- Passed to the app manager: `app.New(m.config, ...)`
- Propagated to services via context during `Init(ctx)` using the `confutil` package
- Services read dynamic config via `confutil.FromContext(ctx)`
- Hot-reload supported via `config.WatchConfig()`

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

- **Configuration Management**: Instance-based Viper with YAML, env var support, and hot-reload
- **Config Propagation**: Configuration passed to services via context during `Init(ctx)`
- **Service Orchestration**: Automatic dependency resolution with topological sort
- **Health Monitoring**: Periodic liveness/readiness probes with auto-restart on failure
- **Graceful Shutdown**: Configurable timeout for cleanup on SIGINT/SIGTERM
- **Signal Handling**: Built-in handlers for SIGINT, SIGTERM, SIGUSR1, SIGUSR2
- **Per-Service Operations**: Init, start, stop, and restart individual services at runtime
- **Pub/Sub Messaging**: Inter-service communication via hierarchical topic bus (memory, Redis, Kafka drivers)
- **Database Integration**: Built-in migration and connection pooling
- **API Flexibility**: Support for multiple servers, TLS, throttling, trailing slash normalization
- **Debugging Tools**: Signal-based introspection and pprof profiling
- **Logging**: Structured logging with file rotation
- **Production Ready**: Panic recovery and error handling

## Best Practices

1. **Configuration**: Use instance-based Viper, env vars for secrets, YAML for structure
2. **Dependencies**: Declare all service dependencies explicitly
3. **Error Handling**: Always check and log initialization errors
4. **Graceful Shutdown**: Configure `WithShutdownTimeout()` and test SIGINT handling
5. **Health Probes**: Implement `Liveness` and `Readiness` interfaces on services for auto-monitoring
6. **Monitoring**: Configure `WithMonitorInterval()` for health checks, use pprof for profiling
7. **Logging**: Set appropriate log levels (Debug in dev, Info+ in production)
8. **Database**: Configure connection pools based on load

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
3. **Initialize in Init(ctx)** (pass required dependencies as constructor arguments):
   ```go
   m.myService = myservice.New(
       m.db,  // Required dependencies first
       myservice.WithLogger(m.log),  // Optional configs
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
- [Service Controller](../../../../pkg/services/app/)
- [Pub/Sub Bus](../../../../pkg/services/pubsub/)
- [Database Manager](../../../../pkg/services/db/)
- [Viper Configuration](https://github.com/spf13/viper)
