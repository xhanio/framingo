---
name: framingo
description: Use when working with Framingo (`github.com/xhanio/framingo`) Go code — creating services, registering with the supervisor, configuring HTTP servers/routers, the database service, pub/sub messaging, or implementing service lifecycle interfaces. Triggers on mentions of framingo, service lifecycle, supervisor, handler groups, or any framingo package imports.
compatibility: Requires Go 1.24+. Framework module is github.com/xhanio/framingo.
metadata:
  author: xhanio
  version: "1.0"
---

# Framingo - Service-Oriented Go Framework

## Overview

Framingo is a modular, production-ready Go framework for building HTTP API applications with service lifecycle management, database integration, pub/sub messaging, and health monitoring.

## When to Use

- Creating a new framingo service or `Manager` interface
- Registering services with the supervisor or wiring service dependencies
- Implementing the lifecycle interfaces (`Service`, `Initializable`, `Daemon`, `Liveness`, `Readiness`, `Debuggable`)
- Configuring HTTP servers, routers, handler groups, middlewares, or WebSocket routes
- Working with the database service (`db.Manager`, transactions, migrations)
- Publishing or subscribing on the pub/sub bus
- Reading config via `confutil.FromContext(ctx)` or shaping the app's YAML config
- Touching any `github.com/xhanio/framingo/...` import

**When NOT to use**: generic Go questions, libraries unrelated to `xhanio/framingo`, or non-Go codebases.

## Quick Reference

| Concern | Package | Interface / Key Type |
|---|---|---|
| Service orchestration | `pkg/services/supervisor` | `supervisor.Manager` |
| Database | `pkg/services/db` | `db.Manager` |
| HTTP API server | `pkg/services/api/server` | `server.Manager`, `api.Router`, `api.Middleware` |
| Pub/Sub | `pkg/services/pubsub` | `pubsub.Manager`, `pubsub.Message` |
| Logging | `pkg/utils/log` | `log.Logger` |
| Errors | `github.com/xhanio/errors` | `errors.Newf`, `errors.Wrap`, category sentinels |
| Config | `pkg/utils/confutil` | `confutil.FromContext(ctx)` |

For deep reference:
- API server: see [api-server.md](api-server.md)
- Errors: see [errors-reference.md](errors-reference.md)
- Config YAML: see [config-reference.md](config-reference.md)
- Package layout: see [package-layout.md](package-layout.md)

## Architecture

```
CLI / Config (cobra + viper)
    |
Supervisor (lifecycle orchestration, dependency resolution, health monitoring)
    |
Services (DB, API Server, PubSub, Planner, custom services)
    |
Types (common interfaces, API types, ORM base types)
```

**Module**: `github.com/xhanio/framingo`

## Core Concepts

### Service Lifecycle Interfaces

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

### Supervisor

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

The manager:
- Resolves dependencies via topological sort
- Calls `Init(ctx)` on `Initializable` services in dependency order
- Calls `Start(ctx)` on `Daemon` services
- Monitors `Liveness` and `Readiness` probes
- Auto-restarts services that fail liveness checks
- Exposes `Restart(ctx) error` for explicit runtime restart of the whole service graph
- Handles graceful shutdown via OS signals

### Configuration Pattern

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

Priority: CLI flags > env vars > YAML file > defaults.

For the full annotated YAML template (log, db, api, pprof, custom service keys) and dynamic-key notes, see [config-reference.md](config-reference.md).

## Database Service

Located in `pkg/services/db`. Supports PostgreSQL, MySQL, SQLite, ClickHouse.

### Creating a DB Manager

```go
import "github.com/xhanio/framingo/pkg/services/db"

dbMgr := db.New(
    db.WithType(db.Postgres),       // or db.MySQL, db.SQLite, db.Clickhouse
    db.WithDataSource(db.Source{
        Host:     "localhost",
        Port:     5432,
        User:     "app",
        Password: "secret",
        DBName:   "mydb",
        Secure:   false,
        Params:   map[string]string{"sslmode": "disable"},
    }),
    db.WithConnection(10, 5, 5*time.Minute, 0), // maxOpen, maxIdle, maxLifetime, maxIdleTime
    db.WithMigration("migrations", 0),            // directory, target version (0 = latest)
    db.WithLogger(logger),
)
```

### Manager Interface

```go
type Manager interface {
    common.Service
    common.Initializable
    common.Debuggable
    ORM() *gorm.DB                                                              // raw GORM access
    DB() *sql.DB                                                                // raw sql.DB access
    FromContext(ctx context.Context) *gorm.DB                                   // context-aware (extracts TX if present)
    FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
    Cleanup(schema bool) error                                                  // truncate tables (schema=true drops schema)
    Reload() error                                                              // drop + re-migrate
    Transaction(ctx context.Context, fn func(ctx context.Context) error, opts ...*sql.TxOptions) error
}
```

### Context-Aware Queries and Transactions

Always use `FromContext` in service/handler code to support transactions:

```go
// Simple query - automatically uses transaction if one exists in context
func (s *myService) GetUser(ctx context.Context, id string) (*User, error) {
    db := s.dbMgr.FromContext(ctx)
    var user User
    return &user, db.First(&user, "id = ?", id).Error
}

// Transaction - wraps fn in a DB transaction, rolls back on error or panic
err := s.dbMgr.Transaction(ctx, func(txCtx context.Context) error {
    // All FromContext calls within this fn use the same transaction
    if err := s.createOrder(txCtx, order); err != nil {
        return err // triggers rollback
    }
    return s.updateInventory(txCtx, order.Items) // committed if nil
})

// Manual context wrapping (advanced)
tx := dbMgr.ORM().Begin()
txCtx := db.WrapContext(ctx, tx)
```

### Dynamic Config Keys

During `Init(ctx)`, the DB manager reads these from Viper:
- `db.connection.max_open` - max open connections
- `db.connection.max_idle` - max idle connections
- `db.connection.max_lifetime` - connection max lifetime
- `db.connection.max_idle_time` - idle connection max lifetime

### ORM Base Types

Located in `pkg/types/orm`:

```go
// Records must implement this generic interface
type Record[T comparable] interface {
    GetID() T
    GetErased() bool
    GetVersion() int64
    TableName() string
}

// For referential integrity tracking
type Referenced[T comparable] interface {
    References() []Reference[T]
}
```

## API Server

Located in `pkg/services/api/server`. Built on Echo. Routes are defined declaratively: each `api.Router` ships an embedded `router.yaml` plus a `Handlers()` map; the server manager binds them at registration time.

```go
import "github.com/xhanio/framingo/pkg/services/api/server"

srvMgr := server.New(server.WithLogger(logger))
srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/"))

srvMgr.RegisterMiddlewares(authMW, corsMW)   // must come before routers
srvMgr.RegisterRouters(userRouter, orderRouter)
```

For the full registration flow, router/middleware contracts, YAML format, handler key format, WebSocket handling, and middleware resolution, see [api-server.md](api-server.md).

## Pub/Sub Messaging

Located in `pkg/services/pubsub`. Supports Memory, Redis, and Kafka backends.

### Message Interfaces

```go
// Typed messages
type Message interface { Kind() string }
type MessageHandler interface { HandleMessage(ctx context.Context, e Message) error }

// Raw messages
type RawMessageHandler interface { HandleRawMessage(ctx context.Context, kind string, payload any) error }
```

### Features
- Hierarchical topic subscriptions
- Non-self-delivery (publishers don't receive their own messages)
- Both typed (`Message`) and raw `(kind, payload)` dispatch

## Logging

Located in `pkg/utils/log`. Built on zap.

```go
import "github.com/xhanio/framingo/pkg/utils/log"

// Use service-scoped logger
logger := log.Default.By(myService)  // prefixes logs with service name
logger.Infof("started on port %d", port)
logger.Debugf("processing request %s", id)
```

## Context Keys

Defined in `pkg/types/common/context.go`:
- `_config` - Viper config instance
- `_tx` - Database transaction (`*gorm.DB`)
- `_db` - Database reference
- `_logger` - Logger instance
- `_credential`, `_session`, `_namespace` - Auth context
- `_api_request_info`, `_api_response_info`, `_api_error` - API context

## Creating a New Service

Implement the interfaces you need. Always use an **unexported struct** with an **exported interface** and factory function — this is a strict convention throughout framingo.

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

// Factory function returns the exported interface
func New(database db.Manager, opts ...Option) Manager {
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

## Error Handling

All errors in framingo MUST use `github.com/xhanio/errors` — never `fmt.Errorf` or stdlib `errors`. The API server's error handler routes on `xhanio/errors` categories to set the response HTTP status.

```go
return errors.NotFound.Newf("user %s not found", id)
if err := s.db.FromContext(ctx).Create(u).Error; err != nil {
    return errors.Wrapf(err, "failed to create user %s", u.Name)
}
```

For the full category table, wrapping rules, combining, checking, and custom categories, see [errors-reference.md](errors-reference.md).

## Package Organization

All application packages MUST follow the categorized `pkg/` layout. Every category directory is a grouping folder only — Go source files always live in subdirectories, never at a category root.

Categories:
- `components/` — application wiring (cmd, server daemons)
- `services/` — business logic, one `Manager` interface per service
- `routers/` — HTTP route handlers (`router.go`, `router.yaml`, `handler.go`)
- `middlewares/` — `api.Middleware` implementations
- `types/api/`, `types/entity/`, `types/orm/` — request, business, and DB types kept strictly separate
- `utils/` — stateless shared helpers

For the full category rules, type-separation example, server component file structure, and import organization, see [package-layout.md](package-layout.md).

## Common Mistakes

- `errors.New("msg")` does not compile — use `errors.Newf("msg")`. `New` takes functional options, not a message.
- Never return raw `err` — always `errors.Wrap(err)` or `errors.Wrapf(err, "context")`. Raw returns drop the stack trace.
- Never use `fmt.Errorf` or stdlib `errors.New` — the API server's error handler routes on `xhanio/errors` categories to set HTTP status.
- Don't place `.go` files at a `pkg/` category root — every category is a grouping folder; code lives in subdirectories (`pkg/services/user/`, not `pkg/services/foo.go`).
- A router's `prefix` MUST match the package's domain (e.g., `routers/user/` → `/users`), not an arbitrary path.
- Don't use the global Viper singleton — use the instance passed via `context.Context` (`confutil.FromContext(ctx)`).
- After `echo.Shutdown` is called, the same echo instance can't be reused — the framework's api server rebuilds it on `Init`, but custom services must do the same if they wrap net/http servers.

## Key Patterns

1. **Functional Options** - All services use `With*()` option functions for configuration
2. **Interface Composition** - Combine small interfaces (`Service` + `Initializable` + `Daemon`) as needed
3. **Context Propagation** - Config and transactions flow through `context.Context`
4. **Dependency Declaration** - Services declare dependencies via `Dependencies()`, manager resolves order
5. **Categorized Packages** - ALL code under `pkg/` must follow the category structure above
6. **Type Separation** - Each domain concept has `api/`, `entity/`, and `orm/` representations
7. **Error Handling** - ALWAYS use `github.com/xhanio/errors`, NEVER use `fmt.Errorf` or stdlib `errors`
8. **Import Order** - Three groups: stdlib, third-party, project — always separated by blank lines

## Data Structures

Available in `pkg/structs/`:
- `graph/` - Generic directed graph with topological sort
- `buffer/` - Ring buffer with pooling
- `queue/` - FIFO queue
- `staque/` - Hybrid stack/queue with priority
- `trie/` - Prefix tree for string matching
- `lease/` - Time-based lease management
