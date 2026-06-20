# Example Server Component

This directory contains a complete server component implementation demonstrating how to build a production-ready HTTP API server using the Framingo framework.

## Overview

The `example` server component is a comprehensive implementation that orchestrates all parts of a Framingo application, including:
- Instance-based Viper configuration with hot-reload support
- Database connectivity and migration
- Pub/sub message bus for inter-service communication
- Service lifecycle management with context-based config propagation
- HTTP API server setup with routers and middlewares
- Built-in signal handling (SIGINT, SIGTERM, SIGHUP, SIGUSR1, SIGUSR2)
- Health monitoring with liveness/readiness probes and auto-restart
- Profiling support (pprof)

This component serves as the main entry point for your application and demonstrates best practices for assembling a complete server.

## Structure

```
example/
├── manager.go    # Manager struct (fields for log, db, bus, repository, system/business services, api, supervisor)
├── model.go      # Server interface
├── config.go     # Viper configuration setup and loading
├── service.go    # Service instance creation (logger, db, bus, repository, system services, business services, api)
├── lifecycle.go  # Init/Start/Stop, supervisor registration order
├── signal.go     # OS signal handling (SIGINT, SIGTERM, SIGHUP, SIGUSR1, SIGUSR2)
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

Service instance creation (in `initServices`):
- Logger initialization with file rotation
- Service controller (`supervisor.Manager`) creation
- Infra services: database manager (with pooling/migration), pub/sub bus (memory/Redis/Kafka driver), repository
- System services: `user`, `role`, `organization`, `certificate`, `auth` (DI wired — auth depends on user, all system services depend on repository)
- Business services: `example` (depends on repository)
- API server manager with per-server endpoint, throttle, and TLS configuration

### [lifecycle.go](lifecycle.go)

Server lifecycle orchestration:
- Service registration order: infra (`db`) → system (`bus`, `repository`, `user`, `role`, `organization`, `certificate`, `auth`) → business (`example`) → topo sort → `api` last
- Pub/sub bus subscriptions for all services on hierarchical topics
- Service initialization and post-initialization (API wiring via `initAPI`)
- Startup flow with pprof and signal handling
- Graceful shutdown

### [api.go](api.go)

Handles API-specific initialization:
- Middleware registration: `deflate`, `authnuser` (user JWT/session, needs `auth` + `role`), `authnagent`, `authz` (RBAC, needs `role`), `feature`
- Router registration: `example`, `auth`, `user`, `role`, `certificate`
- Integration with [pkg/middlewares](../../middlewares/)
- Integration with [pkg/routers](../../routers/)

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
   - Create service controller: `supervisor.New(m.config, ...)`
   - Create infra services: database manager, pub/sub bus, repository
   - Create system services: `user`, `role`, `organization`, `certificate`, `auth` (DI between them)
   - Create business services (e.g. `example` with repository dependency)
   - Create API server manager with per-server endpoint, throttle, and TLS configuration

3. **Register and Wire Services** (via [lifecycle.go](lifecycle.go))
   - Register infra → system → business services in order with the supervisor
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

1. **Guard Against Double Start**
   - Returns early with a warning log if the manager has already been started and not stopped

2. **Enable pprof (if configured)**
   - Starts pprof HTTP server on configured port in a background goroutine

3. **Start All Services**
   - Launches services in dependency order via the supervisor
   - Health monitoring (liveness/readiness probes, auto-restart) runs inside the supervisor once services are started

4. **Listen for Signals (blocks)**
   - Derives a cancellable child context (`m.ctx`, `m.cancel`) from the caller's context
   - Calls `listenSignals(m.ctx)` synchronously — this is the blocking call that keeps `Start()` running
   - Returns when either a shutdown signal is received or `m.Stop()` is called programmatically

### Shutdown Flow

The `Stop()` method:

1. Stops all services in reverse dependency order via the supervisor (closes database connections and other resources owned by services)
2. Cancels `m.cancel` to unblock `listenSignals` (and therefore `Start()`)
3. Resets `m.cancel` to `nil` so the manager can be restarted

Shutdown can be triggered in two ways:

- **By signal**: `listenSignals` receives `SIGINT`/`SIGTERM` and calls `m.Stop(true)` itself, then returns.
- **Programmatically**: external code calls `m.Stop(wait)`; the context cancellation wakes `listenSignals`, which logs the shutdown and returns.

Both paths converge on the same `Stop()` logic — there is a single shutdown code path.

### Restart Flow

`SIGHUP` triggers `m.services.Restart(ctx)` on the supervisor (see [signal.go](signal.go)):

1. Supervisor stops all services in reverse dependency order, then re-initializes and re-starts them.
2. The api server rebuilds its underlying `echo` instance on each `Init` because `net/http` forbids reusing a `Server` after `Shutdown` — without this, the second start would fail with `http: Server closed`.
3. Routers and middlewares are re-registered as part of the api server's re-init path.

## Service Dependencies

The server component manages several layers of services:

### Utility Services
- **Logger**: Application-wide logging
- **Database Manager**: Connection pool and migrations

### Infra Services
- **Pub/Sub Bus**: Inter-service message bus ([pkg/services/pubsub](../../../../pkg/services/pubsub/))
- **Repository**: Data access layer on top of the database manager (see [pkg/services/repository](../../services/repository/))

### System Services
- **User / Role / Organization / Certificate / Auth**: Identity, RBAC, org tree, cert storage, and authentication (see [pkg/services/system](../../services/system/))

### Business Services
- **Example Service**: Custom business logic (see [pkg/services/example](../../services/example/))

### API Services
- **Server Manager**: HTTP API server ([pkg/services/api/server](../../../../pkg/services/api/server/))
- **Routers**: HTTP route handlers — `example`, `auth`, `user`, `role`, `certificate` (see [pkg/routers](../../routers/))
- **Middlewares**: Request pipeline — `deflate`, `authnuser`, `authnagent`, `authz`, `feature` (see [pkg/middlewares](../../middlewares/))

### Service Controller
- **Controller Manager**: Orchestrates service lifecycle ([pkg/services/supervisor](../../../pkg/services/supervisor/))

The controller automatically resolves dependencies using topological sorting and manages health monitoring:

```go
// Create service controller with config for propagation to services
m.services = supervisor.New(m.config,
    supervisor.WithLogger(m.log),
    supervisor.WithShutdownTimeout(30*time.Second),     // Graceful shutdown timeout
    supervisor.WithMonitorInterval(10*time.Second),      // Health check interval
    supervisor.WithRestartPolicy(3),                     // Max restart attempts
    supervisor.WithRestartDelay(5*time.Second),          // Delay between restarts
)

// Register infra services
m.services.Register(m.db)

// Register system services
m.services.Register(
    m.bus,
    m.repository,
    m.user,
    m.role,
    m.organization,
    m.certificate,
    m.auth,
)

// Register business services
m.services.Register(m.example)

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
    middlewares := []api.Middleware{
        deflate.New(),
        authnuser.New(m.auth, m.role),
        authz.New(m.role),
        // authnagent.New(...), feature.New(...) — register as needed
    }
    routers := []api.Router{
        exampleRouter.New(m.example, m.log),
        authRouter.New(m.auth, m.log),
        userRouter.New(m.user, m.role, m.auth, m.log),
        roleRouter.New(m.role, m.log),
        certRouter.New(m.certificate, m.log),
    }
    if err := m.api.RegisterMiddlewares(middlewares...); err != nil {
        return errors.Wrap(err)
    }
    if err := m.api.RegisterRouters(routers...); err != nil {
        return errors.Wrap(err)
    }
    return nil
}
```

This connects:
- Middlewares from [pkg/middlewares](../../middlewares/) (`deflate`, `authnuser`, `authnagent`, `authz`, `feature`)
- Routers from [pkg/routers](../../routers/) (`example`, `auth`, `user`, `role`, `certificate`)
- System services from [pkg/services/system](../../services/system/) and business services from [pkg/services/example](../../services/example/)

## Signal Handling

Signal handling is managed by the server component in [signal.go](signal.go).

### Signal Handlers

| Signal | Action |
|--------|--------|
| `SIGINT` / `SIGTERM` | Graceful shutdown (calls `m.Stop(true)`, which stops all services and cancels the manager context) |
| `SIGHUP` | Restart all services via `m.services.Restart(ctx)` (supervisor stops, re-initializes, and re-starts; api server rebuilds its echo instance) |
| `SIGUSR1` | Print service info (name, state, alive/ready, uptime) |
| `SIGUSR2` | Print all goroutine stack traces |

### Usage

```bash
# Graceful shutdown
kill -INT <pid>   # or press Ctrl+C

# Restart all services (reload config + reinit)
kill -HUP <pid>

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
- Passed to the supervisor: `supervisor.New(m.config, ...)`
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
- **Signal Handling**: Built-in handlers for SIGINT, SIGTERM, SIGHUP (restart all services), SIGUSR1, SIGUSR2
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

1. **Create the router** (see [pkg/routers](../../routers/))
2. **Register in [api.go](api.go)**:
   ```go
   routers := []api.Router{
       exampleRouter.New(m.example, m.log),
       myrouter.New(m.myService, m.log),
   }
   ```

### Adding a New Middleware

1. **Create the middleware** (see [pkg/middlewares](../../middlewares/))
2. **Register in [api.go](api.go)**:
   ```go
   middlewares := []api.Middleware{
       deflate.New(),
       authnuser.New(m.auth, m.role),
       authz.New(m.role),
       mymiddleware.New(),
   }
   ```

## See Also

- [Example Services](../../services/example/)
- [System Services](../../services/system/)
- [Repository Service](../../services/repository/)
- [Routers](../../routers/)
- [Middlewares](../../middlewares/)
- [API Server Manager](../../../../pkg/services/api/server/)
- [Service Controller (supervisor)](../../../../pkg/services/supervisor/)
- [Pub/Sub Bus](../../../../pkg/services/pubsub/)
- [Database Manager](../../../../pkg/services/db/)
- [Viper Configuration](https://github.com/spf13/viper)
