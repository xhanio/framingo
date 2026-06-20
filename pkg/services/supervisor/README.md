# Supervisor Service

The `supervisor` service orchestrates the lifecycle of every other service in a Framingo application: registration, dependency resolution, initialization, startup, health monitoring, restart, and graceful shutdown.

Import path: `github.com/xhanio/framingo/pkg/services/supervisor`

## Table of Contents

- [Service Interfaces](#service-interfaces)
- [Writing a Service](#writing-a-service)
- [Managing Services with the Supervisor](#managing-services-with-the-supervisor)
- [Dynamic Configuration](#dynamic-configuration)
- [Health Probes](#health-probes)
- [Health Monitoring and Auto-Restart](#health-monitoring-and-auto-restart)
- [Per-Service Operations](#per-service-operations)
- [Service Stats and Debugging](#service-stats-and-debugging)
- [Shutdown Timeout](#shutdown-timeout)
- [Testing Services](#testing-services)

## Service Interfaces

Framingo services are composed from a set of interfaces defined in [`pkg/types/common/service.go`](../../types/common/service.go). A service implements only the interfaces it needs.

### Required: `common.Service`

Every service must implement this. It identifies the service and declares its dependencies.

```go
type Service interface {
    Name() string
    Dependencies() []Service
}
```

### Optional: `common.Initializable`

For services that require setup before running. `Init` receives a `context.Context` that carries the application's `*viper.Viper` configuration (accessible via `confutil.FromContext(ctx)`).

`Init` is called on **first startup and on every restart**, making it the appropriate place to load dynamic configuration that may change between runs.

```go
type Initializable interface {
    Init(ctx context.Context) error
}
```

### Optional: `common.Daemon`

For services that run continuously in the background. `Start` launches the service and `Stop` shuts it down.

```go
type Daemon interface {
    Start(ctx context.Context) error
    Stop(wait bool) error
}
```

### Optional: `common.Liveness`

Liveness probes indicate whether a service is alive. A liveness failure triggers an **automatic restart** by the health monitor.

```go
type Liveness interface {
    Alive() error
}
```

### Optional: `common.Readiness`

Readiness probes indicate whether a service is ready to accept work. A readiness failure is **reported but does not trigger a restart**.

```go
type Readiness interface {
    Ready() error
}
```

### Optional: `common.Debuggable`

For services that can print diagnostic information.

```go
type Debuggable interface {
    Info(w io.Writer, debug bool)
}
```

### Interface Combinations

Not all interfaces are required. The supervisor adapts its behavior based on what a service implements:

| Interfaces | Behavior |
|-----------|----------|
| `Service` only | Registered in dependency graph but no lifecycle management |
| `Service` + `Initializable` | Initialized but not started/stopped (e.g. config loaders) |
| `Service` + `Initializable` + `Daemon` | Full lifecycle: init, start, stop |
| Any of above + `Liveness` | Health monitor checks liveness, restarts on failure |
| Any of above + `Readiness` | Health monitor tracks readiness state |

## Writing a Service

A standard Framingo service consists of three files:

```
myservice/
├── manager.go   # Implementation
├── model.go     # Interface definition
└── option.go    # Functional options
```

### model.go

Define the public interface by composing the common interfaces with your business methods:

```go
package myservice

import (
    "context"
    "github.com/xhanio/framingo/pkg/types/common"
)

type Manager interface {
    common.Service
    common.Initializable
    common.Daemon

    DoWork(ctx context.Context, input string) (string, error)
}
```

### option.go

Use the standard `apply()` pattern for functional options. This pattern allows options to be re-applied during restart for dynamic reconfiguration.

```go
package myservice

import "github.com/xhanio/framingo/pkg/utils/log"

type Option func(*manager)

func (m *manager) apply(opts ...Option) {
    for _, opt := range opts {
        opt(m)
    }
}

func WithLogger(logger log.Logger) Option {
    return func(m *manager) {
        m.log = logger.By(m)
    }
}

// Options for values that may change on restart via dynamic config
func WithRateLimit(rps float64) Option {
    return func(m *manager) {
        m.rateLimit = rps
    }
}
```

### manager.go

Implement the service. Required dependencies are constructor arguments, optional ones are options.

```go
package myservice

import (
    "context"
    "path"
    "sync"

    "github.com/xhanio/framingo/pkg/services/db"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/confutil"
    "github.com/xhanio/framingo/pkg/utils/log"
    "github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
    name string
    log  log.Logger

    // required dependency
    db db.Manager

    // configurable fields
    rateLimit float64

    // runtime state
    ctx    context.Context
    cancel context.CancelFunc
    wg     *sync.WaitGroup
}

// New creates the service. Required dependencies are arguments, not options.
func New(database db.Manager, opts ...Option) Manager {
    m := &manager{
        log: log.Default,
        db:  database,
        wg:  &sync.WaitGroup{},
    }
    m.apply(opts...)
    m.log = m.log.By(m)
    return m
}

// Name returns the service name, auto-derived from the package path.
func (m *manager) Name() string {
    if m.name == "" {
        m.name = path.Join(reflectutil.Locate(m))
    }
    return m.name
}

// Dependencies declares required services. The supervisor uses this
// for topological sorting — dependencies are always started first.
func (m *manager) Dependencies() []common.Service {
    return []common.Service{m.db}
}

// Init is called on startup and on every restart.
// Read dynamic configuration from context here.
func (m *manager) Init(ctx context.Context) error {
    config := confutil.FromContext(ctx)
    m.apply(
        WithRateLimit(config.GetFloat64("myservice.rate_limit")),
    )
    m.log.Infof("initialized with rate_limit=%.1f", m.rateLimit)
    return nil
}

// Start launches the service's background work.
func (m *manager) Start(ctx context.Context) error {
    if m.cancel != nil {
        return nil // already started
    }
    ctx, cancel := context.WithCancel(context.Background())
    m.cancel = cancel
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        // service loop
        <-ctx.Done()
        m.log.Infof("service %s stopped", m.Name())
    }()
    return nil
}

// Stop shuts down the service. If wait is true, blocks until cleanup is done.
func (m *manager) Stop(wait bool) error {
    if m.cancel == nil {
        return nil // already stopped
    }
    m.cancel()
    if wait {
        m.wg.Wait()
    }
    m.cancel = nil
    return nil
}

func (m *manager) DoWork(ctx context.Context, input string) (string, error) {
    // business logic using m.db, m.rateLimit, etc.
    return input, nil
}
```

### Key Patterns

1. **Required dependencies as constructor arguments** — guarantees they're always present
2. **Optional config as functional options** — allows flexibility without breaking the constructor
3. **`apply()` method on the struct** — enables re-applying options during restart
4. **`confutil.FromContext(ctx)` in `Init`** — reads dynamic config from the Viper instance
5. **`reflectutil.Locate(m)` for Name()** — auto-derives the service name from the package path
6. **Idempotent Start/Stop** — guard against double-start and double-stop
7. **`sync.WaitGroup` for graceful shutdown** — ensures goroutines finish before `Stop` returns

## Managing Services with the Supervisor

The `supervisor.Manager` orchestrates service lifecycle: registration, dependency resolution, initialization, startup, health monitoring, and shutdown. Its business surface is published as [`model.Supervisor`](../../types/model/supervisor.go) so other packages can depend on the interface without importing the implementation.

### Creating the Supervisor

The supervisor requires a `*viper.Viper` config instance. This config is propagated to all services during `Init(ctx)` via context.

```go
import (
    "github.com/spf13/viper"
    "github.com/xhanio/framingo/pkg/services/supervisor"
)

config := viper.New()
config.SetConfigFile("config.yaml")
config.ReadInConfig()

services := supervisor.New(config,
    supervisor.WithLogger(logger),
)
```

### Registering Services

Register services and their dependencies. Dependencies declared via `Dependencies()` are automatically registered.

```go
dbManager := db.New(...)
myService := myservice.New(dbManager, myservice.WithLogger(logger))
apiServer := server.New(...)

// Dependencies are auto-registered from Dependencies()
services.Register(dbManager, myService)
```

### Dependency Resolution

Call `TopoSort()` to resolve the dependency graph. Services will be initialized and started in dependency order, and stopped in reverse order.

```go
if err := services.TopoSort(); err != nil {
    // circular dependency or missing service
    log.Fatal(err)
}

// Register services that must start last (e.g. API server)
services.Register(apiServer)
```

### Lifecycle: Init, Start, Stop, Restart

```go
// Init all services (in dependency order)
// Config is propagated to services via context
if err := services.Init(ctx); err != nil {
    log.Fatal(err)
}

// Start all daemon services (in dependency order)
// Also starts health monitor if configured
if err := services.Start(ctx); err != nil {
    log.Fatal(err)
}

// Restart the entire supervisor: Stop(true) → Init(ctx) → Start(ctx).
// Useful for top-level config reload without exiting the process.
if err := services.Restart(ctx); err != nil {
    log.Error(err)
}

// Stop all services (in reverse dependency order)
if err := services.Stop(true); err != nil {
    log.Error(err)
}
```

### Lifecycle Behavior

During `Init`:
- Services are initialized in topological (dependency) order
- If a dependency fails to initialize, dependent services are skipped
- Services that don't implement `Initializable` are marked `Initialized=true` so dependents proceed
- `*viper.Viper` config is injected into the context via `confutil.WrapContext`

During `Start`:
- Only services implementing `Daemon` are started
- Health monitor goroutine starts if `WithMonitorInterval` is configured
- Double-start is a safe no-op

During `Restart` (supervisor-level):
- Calls `Stop(true)` to drain all services in reverse order — a `Stop` error is logged but does not abort the restart
- Calls `Init(ctx)` to re-read dynamic config and re-initialize every service
- Calls `Start(ctx)` to bring services back up
- `Init` or `Start` errors abort and are returned to the caller
- Per-service `Restarts` counters are not bumped (use `RestartService` for that)

During `Stop`:
- Services are stopped in **reverse** dependency order
- An optional shutdown timeout prevents hanging (`WithShutdownTimeout`)
- Double-stop is a safe no-op

## Dynamic Configuration

Services read dynamic configuration from context during `Init(ctx)`. The supervisor wraps the `*viper.Viper` instance into the context using `confutil.WrapContext`, and services extract it with `confutil.FromContext(ctx)`.

```go
import "github.com/xhanio/framingo/pkg/utils/confutil"

func (m *manager) Init(ctx context.Context) error {
    config := confutil.FromContext(ctx)
    m.apply(
        WithRateLimit(config.GetFloat64("myservice.rate_limit")),
        WithTimeout(config.GetDuration("myservice.timeout")),
    )
    return nil
}
```

Since `Init` is called on every restart, config changes take effect automatically when a service is restarted — by the supervisor's `Restart`, by `RestartService`, or by the health monitor.

Combined with `config.WatchConfig()` on the Viper instance, this enables hot-reload scenarios.

## Health Probes

### Liveness

Implement `common.Liveness` to enable automatic restart on failure:

```go
func (m *manager) Alive() error {
    if !m.isConnected() {
        return fmt.Errorf("lost connection to upstream")
    }
    return nil
}
```

### Readiness

Implement `common.Readiness` to report whether the service can accept work:

```go
func (m *manager) Ready() error {
    if !m.cacheWarmed {
        return fmt.Errorf("cache not yet warmed")
    }
    return nil
}
```

### Behavior Differences

| Probe | Failure Effect |
|-------|---------------|
| Liveness | Triggers automatic restart (if monitor is configured) |
| Readiness | Sets `Ready=false` in stats, but does **not** restart |

The health monitor also checks the service's basic health (init/start errors, stopped state) before checking liveness and readiness probes.

Health checks run recursively through dependencies — if a dependency fails its liveness check, that failure propagates to dependent services.

## Health Monitoring and Auto-Restart

Configure the health monitor to periodically check services and automatically restart those that fail liveness:

```go
services := supervisor.New(config,
    supervisor.WithLogger(logger),
    supervisor.WithMonitorInterval(10*time.Second),  // check every 10s
    supervisor.WithRestartPolicy(3),                 // max 3 restart attempts
    supervisor.WithRestartDelay(5*time.Second),      // wait 5s before restart
)
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMonitorInterval(d)` | Health check interval. Set to 0 to disable monitoring. | 0 (disabled) |
| `WithRestartPolicy(n)` | Max restart attempts per service. 0 disables restart, -1 is unlimited. | 0 (disabled) |
| `WithRestartDelay(d)` | Delay before attempting a restart. | 0 |

### Restart Behavior

When a service fails its liveness check:

1. The monitor logs the failure
2. Waits for `restartDelay` (if configured)
3. Checks if `maxRetries` has been reached
4. Calls `Stop` → `Init` → `Start` on the service
5. Increments `SupervisorStats.Restarts` and updates `SupervisorStats.RestartedAt`

## Per-Service Operations

The supervisor supports operating on individual services at runtime:

```go
// Re-initialize a single service
err := services.InitService(ctx, "myservice")

// Start a single service
err := services.StartService("myservice")

// Stop a single service
err := services.StopService("myservice", true)

// Restart a single service: stop → re-init → start; bumps Restarts counter
err := services.RestartService(ctx, "myservice")

// Restart the whole supervisor: stop all → init all → start all
err := services.Restart(ctx)
```

`RestartService` targets one named service and bumps its `Restarts` counter; `Restart` cycles every registered service through `Stop` → `Init` → `Start` and does not bump per-service counters. Both are useful for runtime management, debugging, or building admin APIs.

## Service Stats and Debugging

### Stats

Retrieve per-service statistics:

```go
stats, err := services.Stats()
for _, stat := range stats {
    fmt.Printf("%-30s init=%-5v started=%-5v ready=%-5v uptime=%s restarts=%d\n",
        stat.Name,
        stat.Initialized,
        stat.Started,
        stat.Ready,
        stat.Uptime(),
        stat.Restarts,
    )
}
```

`Stats()` returns `[]*entity.SupervisorStats` (defined in [`pkg/types/entity/supervisor.go`](../../types/entity/supervisor.go)):

| Field | Description |
|-------|-------------|
| `Name` / `Source` | Service name and the underlying `common.Service` |
| `Initialized` / `InitializedAt` / `InitDuration` | Init state and timing |
| `InitializationErr` | Error from last `Init` call |
| `Started` / `StartedAt` / `StartDuration` | Start state and timing |
| `StartErr` | Error from last `Start` call |
| `Stopped` / `StoppedAt` / `StopDuration` | Stop state and timing |
| `StopErr` | Error from last `Stop` call |
| `LivenessErr` | Error from last `Alive()` call |
| `Ready` / `ReadinessErr` | Readiness state and error |
| `HealthcheckedAt` / `HealthcheckErr` | Last healthcheck timestamp and combined error |
| `Restarts` / `RestartedAt` | Restart count and last restart time |
| `Uptime()` | Duration since last start (0 if stopped) |
| `Healthcheck()` | Combined error from stopped state, init error, and start error |

### Info / Debug Output

The `Info(w, debug)` method prints a formatted table to the given writer:

```
service status
SERVICE                         ALIVE   READY   UPTIME          INIT_ERR   START_ERR   HEALTHCHECK_ERR
pkg/services/db                 true    true    1h23m45s        <nil>      <nil>       <nil>
pkg/services/example            true    true    1h23m44s        <nil>      <nil>       <nil>
pkg/services/api/server         true    true    1h23m43s        <nil>      <nil>       <nil>
```

After the table, each service that implements `Debuggable` has its own `Info` method called to print additional details.

## Shutdown Timeout

Configure a timeout to prevent shutdown from hanging on stuck services:

```go
services := supervisor.New(config,
    supervisor.WithShutdownTimeout(30*time.Second),
)
```

If services don't stop within the timeout, a `DeadlineExceeded` error is returned and the process can exit.

## Testing Services

### Testing a Service in Isolation

```go
func TestMyService(t *testing.T) {
    db := mockDB()
    svc := myservice.New(db, myservice.WithRateLimit(100))

    err := svc.Init(context.Background())
    require.NoError(t, err)

    err = svc.Start(context.Background())
    require.NoError(t, err)

    result, err := svc.DoWork(context.Background(), "test")
    require.NoError(t, err)
    assert.Equal(t, "test", result)

    err = svc.Stop(true)
    require.NoError(t, err)
}
```

### Testing with the Supervisor

```go
func TestServiceLifecycle(t *testing.T) {
    m := supervisor.New(nil, supervisor.WithName("test"))

    db := newMockDB()
    svc := newMockService("svc")
    svc.deps = []common.Service{db}

    m.Register(db, svc)
    require.NoError(t, m.TopoSort())
    require.NoError(t, m.Init(context.Background()))
    require.NoError(t, m.Start(context.Background()))

    stats, err := m.Stats()
    require.NoError(t, err)
    for _, stat := range stats {
        assert.True(t, stat.Initialized)
        assert.True(t, stat.Ready)
    }

    require.NoError(t, m.Stop(true))
}
```

### Testing Health Probes

```go
func TestLivenessRestart(t *testing.T) {
    m := supervisor.New(nil,
        supervisor.WithName("test"),
        supervisor.WithMonitorInterval(50*time.Millisecond),
        supervisor.WithRestartPolicy(2),
    )

    svc := newMockService("svc")
    svc.aliveErr = fmt.Errorf("dead")
    m.Register(svc)

    require.NoError(t, m.TopoSort())
    require.NoError(t, m.Init(context.Background()))
    require.NoError(t, m.Start(context.Background()))

    time.Sleep(200 * time.Millisecond)
    require.NoError(t, m.Stop(true))

    stats, _ := m.Stats()
    assert.Equal(t, 2, stats[0].Restarts)
}
```

## Complete Example

Putting it all together — a server component that wires services with the supervisor:

```go
func (m *server) Init(ctx context.Context) error {
    // Load config
    m.config = viper.New()
    m.config.SetConfigFile(m.configPath)
    m.config.ReadInConfig()
    m.config.WatchConfig()

    // Create logger
    m.log = log.New(log.WithLevel(m.config.GetInt("log.level")))

    // Create supervisor with all options
    m.services = supervisor.New(m.config,
        supervisor.WithLogger(m.log),
        supervisor.WithShutdownTimeout(30*time.Second),
        supervisor.WithMonitorInterval(10*time.Second),
        supervisor.WithRestartPolicy(3),
        supervisor.WithRestartDelay(5*time.Second),
    )

    // Create services (required deps as args, optional as opts)
    m.db = db.New(db.WithLogger(m.log))
    m.myService = myservice.New(m.db, myservice.WithLogger(m.log))
    m.api = server.New(server.WithLogger(m.log))

    // Register and sort
    m.services.Register(m.db, m.myService)
    if err := m.services.TopoSort(); err != nil {
        return err
    }
    m.services.Register(m.api) // API server starts last

    // Init all (config propagated via context)
    if err := m.services.Init(ctx); err != nil {
        return err
    }

    // Register routes
    return m.api.RegisterRouters(myrouter.New(m.myService))
}

func (m *server) Start(ctx context.Context) error {
    return m.services.Start(ctx)
}

func (m *server) Stop(wait bool) error {
    return m.services.Stop(wait)
}
```

## API Reference

### `supervisor.New(config *viper.Viper, opts ...Option) Manager`

Creates a new supervisor. Pass `nil` for config if no config propagation is needed.

### Options

| Option | Description |
|--------|-------------|
| `WithLogger(log.Logger)` | Set the logger |
| `WithName(string)` | Override the auto-derived name |
| `WithShutdownTimeout(time.Duration)` | Max time for graceful shutdown |
| `WithMonitorInterval(time.Duration)` | Health check polling interval (0 = disabled) |
| `WithRestartPolicy(int)` | Max restart attempts (0 = disabled, -1 = unlimited) |
| `WithRestartDelay(time.Duration)` | Delay before restart attempt |

### Interfaces

```go
// supervisor.Manager — the concrete supervisor service.
// Composes the business surface (model.Supervisor) with the lifecycle
// interfaces required to be a service itself.
type Manager interface {
    model.Supervisor
    common.Initializable
    common.Daemon
    common.Debuggable
}

// model.Supervisor — the business surface used by callers (e.g. components).
type Supervisor interface {
    common.Service
    Register(services ...common.Service)
    TopoSort() error
    Services() []common.Service
    Stats() ([]*entity.SupervisorStats, error)
    InitService(ctx context.Context, name string) error
    StartService(name string) error
    StopService(name string, wait bool) error
    RestartService(ctx context.Context, name string) error
    Restart(ctx context.Context) error
}
```

## See Also

- [Common Service Interfaces](../../types/common/service.go)
- [Supervisor Model Interface](../../types/model/supervisor.go)
- [Supervisor Stats Entity](../../types/entity/supervisor.go)
- [Config Context Utility](../../utils/confutil/)
- [Example Service](../../../example/pkg/services/example/)
- [Example Server Component](../../../example/pkg/components/server/example/)
