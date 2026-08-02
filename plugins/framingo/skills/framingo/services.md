# Services — Lifecycle, Supervisor, Config, Authoring

How framingo services are shaped, orchestrated, and configured, and how to
write a new one.

## Service Lifecycle Interfaces

All services compose from interfaces in `pkg/types/common`:

```go
// Required - every service must implement this
type Service interface {
    Named                       // Name() string
    Dependencies() []Service    // declare startup dependencies
}

// Optional lifecycle interfaces - implement as needed
type Initializable interface { Init(ctx context.Context) error }       // setup (called on start AND restart)
type Daemon interface { Start(ctx context.Context) error; Stop(wait bool) error }  // long-running
type Liveness interface { Alive() error }                              // health probe (failure = auto-restart)
type Readiness interface { Ready() error }                             // readiness probe (failure = reported only)
type Debuggable interface { Info(w io.Writer, debug bool) }            // debug output
```

## Supervisor

Orchestrates all services. Located in `pkg/services/supervisor`.

```go
import (
    "github.com/spf13/viper"
    "github.com/xhanio/framingo/pkg/services/supervisor"
)

// Create manager with viper config
mgr := supervisor.New(config,
    supervisor.WithLogger(logger),
    supervisor.WithMonitorInterval(30 * time.Second),
)

// Register services (order doesn't matter - topologically sorted)
mgr.Register(dbService, apiServer, pubsubBus, myService)

// Sort, init, start
mgr.TopoSort()
mgr.Init(ctx)
mgr.Start(ctx)
```

Full signature set — `Register` and `TopoSort` differ in whether they return an error, so don't guess:

```go
type Manager interface {          // = model.Supervisor + Initializable + Daemon + Debuggable
    Name() string
    Dependencies() []common.Service
    Register(services ...common.Service)          // no return value
    TopoSort() error
    Services() []common.Service
    Stats() ([]*entity.SupervisorStats, error)

    Init(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(wait bool) error
    Info(w io.Writer, debug bool)

    InitService(ctx context.Context, name string) error
    StartService(name string) error
    StopService(name string, wait bool) error
    RestartService(ctx context.Context, name string) error
    Restart(ctx context.Context) error             // whole graph
}
```

The manager:
- Resolves dependencies via topological sort
- Calls `Init(ctx)` on `Initializable` services in dependency order
- Calls `Start(ctx)` on `Daemon` services
- Monitors `Liveness` and `Readiness` probes; only liveness failure triggers restart (readiness is reported only)
- Restart behaviour tunes via `WithMonitorInterval`, `WithRestartPolicy(maxRetries)`, `WithRestartDelay`, `WithShutdownTimeout`

**The supervisor does NOT install signal handlers.** There is no `os/signal` anywhere in framingo. Trapping SIGINT/SIGTERM/SIGHUP/SIGUSR1/SIGUSR2 and calling `Stop`/`Restart` is application code you write in `pkg/components/server/<app>/signal.go` — see [package-layout.md](package-layout.md).

## Configuration Pattern

Framingo uses instance-based Viper (NOT the global singleton). Config is propagated via `context.Context`:

```go
import "github.com/xhanio/framingo/pkg/utils/confutil"

// In Init(ctx), read dynamic config:
func (s *myService) Init(ctx context.Context) error {
    config := confutil.FromContext(ctx)
    s.setting = config.GetString("my.setting")
    return nil
}
```

**You never call `WrapContext` yourself in a service.** The supervisor wraps the viper instance it was constructed with (`supervisor.New(config, ...)`) into the context it passes to every service's `Init` — that's the whole delivery mechanism.

`FromContext` **never returns nil**: with no config in the context it returns an empty `viper.New()`, so every getter yields the zero value. No nil check is needed, but it also means a missing config looks like "all defaults" rather than an error — if a setting is mandatory, validate it in `Init`.

Priority: CLI flags > env vars > YAML file > defaults.

For the full annotated YAML template (log, db, api, pprof, custom service keys) and dynamic-key notes, see [config-reference.md](config-reference.md).

## Creating a New Service

Always an **unexported struct** with an **exported interface** and factory function — a strict convention throughout framingo.

### The interface goes in two places

This trips people up because both halves get called "the service interface". Templates: [`_templates/types-model-order.go`](_templates/types-model-order.go) and [`_templates/services-order-model.go`](_templates/services-order-model.go).

| File | Declares | Who depends on it |
|---|---|---|
| `pkg/types/model/order.go` | `model.Order` — business methods + `common.Service`, **no lifecycle** | Routers and other services |
| `pkg/services/order/model.go` | `order.Manager` = `model.Order` + the lifecycle interfaces it implements | Only `pkg/components/server/` wiring |

A router takes `model.Order`, never `order.Manager`. That's what keeps the implementation package-private and stops services importing each other. If a router imports `pkg/services/...`, the split has been skipped.

```go
package myservice

import (
    "context"
    "path"

    "github.com/xhanio/framingo/pkg/services/db"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/log"
    "github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// Exported interface — the public API contract
type Manager interface {
    common.Service
    common.Initializable
    DoSomething(ctx context.Context) error
}

// Unexported struct — implementation detail
type manager struct {
    name string
    log  log.Logger
    db   db.Manager
}

// Factory function returns the exported interface; newManager returns the
// concrete type, so package tests construct *manager without the interface
// in the way.
func New(database db.Manager, opts ...Option) Manager {
    return newManager(database, opts...)
}

func newManager(database db.Manager, opts ...Option) *manager {
    m := &manager{
        log: log.Default,
        db:  database,
    }
    m.apply(opts...)
    m.log = m.log.By(m)
    return m
}

func (m *manager) Name() string {
    if m.name == "" {
        m.name = path.Join(reflectutil.Locate(m))
    }
    return m.name
}

func (m *manager) Dependencies() []common.Service {
    return []common.Service{m.db}
}

func (m *manager) Init(ctx context.Context) error {
    // Called on startup and restart. Read config, set up resources.
    return nil
}
```
