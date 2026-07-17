---
name: framingo
description: Use when working with Framingo (`github.com/xhanio/framingo`) Go code — bootstrapping a new framingo backend project, creating services, registering with the supervisor, configuring HTTP servers/routers, the database service, pub/sub messaging, or implementing service lifecycle interfaces. Triggers on mentions of framingo, "new Go backend", service lifecycle, supervisor, handler groups, or any framingo package imports.
compatibility: Requires Go 1.24+. Framework module is github.com/xhanio/framingo.
metadata:
  author: xhanio
  version: "1.1"
---

# Framingo - Service-Oriented Go Framework

## Overview

Framingo is a modular, production-ready Go framework for building HTTP API applications with service lifecycle management, database integration, pub/sub messaging, and health monitoring.

## When to Use

- **Bootstrapping a new framingo backend project** (see [Starting a New Backend](#starting-a-new-backend) below)
- Creating a new framingo service or `Manager` interface
- Registering services with the supervisor or wiring service dependencies
- Implementing the lifecycle interfaces (`Service`, `Initializable`, `Daemon`, `Liveness`, `Readiness`, `Debuggable`)
- Configuring HTTP servers, routers, handler groups, middlewares, or WebSocket routes
- Working with the database service (`db.Manager`, transactions, migrations)
- Publishing or subscribing on the pub/sub bus
- Reading config via `confutil.FromContext(ctx)` or shaping the app's YAML config
- Touching any `github.com/xhanio/framingo/...` import

**When NOT to use**: generic Go questions, libraries unrelated to `xhanio/framingo`, or non-Go codebases.

## Starting a New Backend

**For any new framingo backend project, fork the `example/` folder.** Don't scaffold from scratch.

`example/` is a self-contained Go module that ships a complete production-shaped service: supervisor wiring, PostgreSQL + migrations, pub/sub + message bus, WebSocket stream, RBAC (auth/user/role/organization/certificate), Echo router with auth & throttle middlewares, structured logging, pprof, signal handling, plus GoPro build templates, a CLI client, Docker image, and Kubernetes manifests.

**Canonical recipe:** [example/QUICKSTART.md](../../example/QUICKSTART.md), sections **"Use This Folder as Your Starting Template"** and **"Forking the Example into Your Own Repo"**. That file owns the fork-and-rename steps (module path, `framingo-example`/`exampleapp`/`examplecli` → your names, directory renames under `build/`, `env/`, `kubernetes/`) and the "Keep vs. rip out" table for pruning system services you don't need.

After forking, use the rest of this skill as the per-package reference for the work you do inside the new project.

## Quick Reference

| Concern | Package | Interface / Key Type |
|---|---|---|
| Service orchestration | `pkg/services/supervisor` | `supervisor.Manager` |
| Database | `pkg/services/db` (+ `db/drivers/`) | `db.Manager`; blank-import a driver subpackage (sqlite/mysql/postgres/clickhouse); sqlite needs `CGO_ENABLED=1` |
| HTTP API server | `pkg/services/api/server` + `pkg/types/api` (alias `fapi`) | `server.Manager`, `fapi.Router`, `fapi.Middleware` |
| Handler request context | **project's own** `<project>/pkg/types/api` (unaliased `api`) | `api.Context` — use as the handler ctx instead of `echo.Context`; not a framingo type, you own it |
| HTTP client | `pkg/services/api/client` | `client.Client` |
| Pub/Sub primitive | `pkg/services/pubsub` (+ `pubsub/driver/`) | `pubsub.Manager`; Memory/Redis/Kafka drivers |
| Message bus (on top of pubsub) | `pkg/services/messagebus` | `messagebus.Manager`, `model.MessageBus`, `model.Messenger` |
| Task planner | `pkg/services/planner` | `planner.Manager`, `model.Planner` |
| Message interfaces | `pkg/types/common` | `Message`, `MessageSender`, `MessageHandler`, `RawMessageHandler` |
| Service interfaces | `pkg/types/model` | `Supervisor`, `Database`, `Pubsub`, `MessageBus`, `Planner` |
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
Services (DB, API Server, PubSub, MessageBus, Planner, custom services)
    |
Types (common interfaces, api types, model interfaces, entity data, orm base)
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

Located in `pkg/services/db`. Each engine (PostgreSQL, MySQL, SQLite, ClickHouse) lives in its own subpackage under `pkg/services/db/drivers/` and self-registers via `db.Register` in `init()`. The core `db` package does not import any concrete GORM/migrate driver — **blank-import the driver subpackage(s) your binary actually needs**, so SQLite-only binaries don't drag in the Postgres/MySQL/ClickHouse client libraries (~17MB saving).

### Creating a DB Manager

```go
import (
    "github.com/xhanio/framingo/pkg/services/db"

    // Blank-import only the engines this binary supports.
    _ "github.com/xhanio/framingo/pkg/services/db/drivers/postgres"
    // _ "github.com/xhanio/framingo/pkg/services/db/drivers/mysql"
    // _ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"      // needs CGO_ENABLED=1
    // _ "github.com/xhanio/framingo/pkg/services/db/drivers/clickhouse"
)

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

If `WithType` names a driver that hasn't been blank-imported, `db.Manager` startup fails with `unsupported db type: <name> (driver not registered — blank-import the corresponding pkg/services/db/drivers/* package)`.

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

Located in `pkg/services/api/server`. Built on Echo. Routes are defined declaratively: each `fapi.Router` ships an embedded `router.yaml` plus a `Handlers()` map; the server manager binds them at registration time. The recommended router layout splits each package into `router.go` (factory + `Handlers()` boilerplate, which typically just calls `api.DiscoverHandlers(r)` and debug-logs the handler count) and `handler.go` (handler bodies). See [api-server.md](api-server.md) for the full pattern.

### Two `api` packages — don't confuse them

Router code imports both. The example project's convention (follow it):

```go
import (
    fapi "github.com/xhanio/framingo/pkg/types/api"  // FRAMEWORK: Router, Middleware, ContextKey*, ErrorBody
    "myapp/pkg/types/api"                            // PROJECT (yours): Context, DiscoverHandlers, DTOs
)
```

- `fapi` — **framingo's** `pkg/types/api`. Has `Router`, `Middleware`, `HandlerKey`, `ErrorBody`, `ContextKeyCredential`. It has **no `Context` type**.
- `api` — **your project's** `pkg/types/api`, which you own and can extend. Defines `Context`, `DiscoverHandlers`, `WrapHandler`, `WrapWebSocket`, and request/response DTOs.

`api.Context` below always means the **project** one. Referring to it as a framingo type is a mistake — framingo ships no such interface.

### Handler signature — use the project `api.Context`

**When defining an API, write handlers as `func(c api.Context) error` — not `func(c echo.Context) error`.** `api.Context` is the interface *your project* defines (canonical version: [`example/pkg/types/api/api.go`](../../example/pkg/types/api/api.go)) that embeds `echo.Context` **and** `context.Context`, and adds project helpers:

```go
// pkg/routers/user/handler.go
import (
    "myapp/pkg/types/api"   // the project wrapper, NOT echo, NOT framingo's pkg/types/api
)

func (r *router) GetUser(c api.Context) error {
    cred, ok := c.Credential()             // project helper — no Get() + type-assert dance
    if !ok {
        return errors.Unauthorized.New()
    }
    u, err := r.svc.Get(c, c.Param("id"))  // c IS a context.Context — pass it straight through
    if err != nil {
        return errors.Wrap(err)
    }
    return c.JSON(http.StatusOK, u)
}
```

Why this is the recommendation:

- **One value, both contracts.** `c` satisfies `echo.Context` (bind/respond) *and* `context.Context`, so service calls take `c` directly — no `c.Request().Context()` unwrap, and cancellation/deadlines propagate for free.
- **Helpers have a home.** `Credential()`, `Session()`, `TraceID()`, and custom binders live on the interface. Adding one later touches your `api.go` only — never every handler signature.
- **You own it.** Because the interface is project-side, extending it needs no framingo change.
- **Zero framework cost.** `api.DiscoverHandlers(r)` reflects over the router and wraps `func(api.Context) error` into the `echo.HandlerFunc` the server registers. The framework still accepts raw `echo.Context` handlers; that's the fallback for third-party code, not the pattern for new handlers.

Same for WebSocket handlers: `func(c api.Context, conn *websocket.Conn) error`.

If a project has no `pkg/types/api/api.go` yet (i.e. it wasn't forked from `example/`), copy [`example/pkg/types/api/api.go`](../../example/pkg/types/api/api.go) into it before writing handlers, and adjust the `entity` import to the project's own.

### `Handlers()` — call `DiscoverHandlers` in each `router.go`

`api.Context` handlers reach the framework through this one hook. **Every router package's `router.go` implements `Handlers()` with the same body**, verbatim across all six routers in `example/pkg/routers/`:

```go
// pkg/routers/user/router.go  — wiring lives here, never in handler.go
func (r *router) Handlers() map[string]any {
    handlers := api.DiscoverHandlers(r)                                    // r = this router, its own methods
    r.log.Debugf("router %s parsed %d handler(s)", r.Name(), len(handlers))
    return handlers
}
```

It reflects over the receiver, keys handlers by method name (matching `func:` in that package's `router.yaml`), and wraps each `func(api.Context) error` into `echo.HandlerFunc`. So adding a handler = write the method in `handler.go` + add a `func:` entry to `router.yaml`. Nothing else.

Don't hand-write the map (`map[string]any{"ListUsers": r.ListUsers}`) — it forces `echo.HandlerFunc` signatures, defeating `api.Context`, and rots on rename. Keep the debug line: methods that don't match a known signature are skipped **silently**, so the count is your only startup signal.

### Wiring the server

```go
import "github.com/xhanio/framingo/pkg/services/api/server"

srvMgr := server.New(server.WithLogger(logger))
srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/"))

srvMgr.RegisterMiddlewares(authMW, corsMW)   // must come before routers
srvMgr.RegisterRouters(userRouter, orderRouter)
```

For the full registration flow, router/middleware contracts, YAML format, handler key format, WebSocket handling, and middleware resolution, see [api-server.md](api-server.md).

## Pub/Sub Messaging

Two layers: the low-level pub/sub primitive (`pkg/services/pubsub`) and a higher-level message bus (`pkg/services/messagebus`) that routes module messages through a single topic on top of it.

### Pub/Sub Primitive

`pkg/services/pubsub` with pluggable drivers under `pkg/services/pubsub/driver/` (Memory, Redis, Kafka).

```go
import (
    "github.com/xhanio/framingo/pkg/services/pubsub"
    "github.com/xhanio/framingo/pkg/services/pubsub/driver"
)

ps := pubsub.New(driver.NewMemory(logger), pubsub.WithLogger(logger))
// ps.Publish(topic, msg); ps.Subscribe(topic, handler); ps.Unsubscribe(topic, handler)
```

Features: hierarchical topic subscriptions, non-self-delivery, both typed and raw dispatch.

#### Slow subscribers

Each subscriber gets a growable pending queue (capped, `driver.WithQueueCap`) drained by its own
goroutine, so a briefly slow consumer loses nothing and never stalls `Publish`. When the queue
fills, the consumer has stopped draining entirely, and `driver.WithOnFull` decides what happens:

```go
// Default: discard the message, count it, log it (throttled).
driver.NewMemory(logger)

// Close the subscriber's channel instead, so it reconnects and resumes from its own cursor.
driver.NewMemory(logger, driver.WithOnFull(driver.DropSubscriber))
```

Pick `DropSubscriber` when consumers can reconnect and replay (a persisted log plus a cursor).
A lost connection is recoverable and visible; a lost message is neither. All three drivers accept
these options, and expose `Dropped()` / `Evicted()` via the optional `driver.Stats` interface,
which `pubsub.Manager.Info` reports.

An in-process bus is never a delivery guarantee — it dies with the process. Durability belongs in
whatever log the consumer replays from.

### Message Bus

`pkg/services/messagebus` wraps a `model.Pubsub` and dispatches via a single well-known topic (`/messages` by default). Modules register once and receive typed (`common.Message`) or raw (`kind`, payload) messages.

```go
import "github.com/xhanio/framingo/pkg/services/messagebus"

mb := messagebus.New(ps, messagebus.WithLogger(logger))

// Register any service: modules implementing common.MessageHandler /
// common.RawMessageHandler are auto-subscribed; others are no-op.
mb.Register(someService)

// Direct channel access (e.g. WebSocket bridge)
messenger, _ := mb.NewMessenger("ws:user-123")
mb.AttachWebSocket(messenger, wsConn) // blocks until conn closes
```

### Message Interfaces

Defined in `pkg/types/common` (not in `pubsub`/`messagebus`):

```go
// Typed messages
type Message interface { Kind() string }
type MessageHandler interface { HandleMessage(ctx context.Context, e Message) error }

// Raw messages
type RawMessageHandler interface { HandleRawMessage(ctx context.Context, kind string, payload any) error }

// Senders
type MessageSender    interface { SendMessage(ctx context.Context, m Message) error }
type RawMessageSender interface { SendRawMessage(ctx context.Context, kind string, payload any) error }
```

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
- `_trace` - Trace context
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
- `components/` — application wiring (cmd, server daemons, client SDKs)
- `services/` — business logic, one `Manager` interface per service
- `routers/` — HTTP route handlers (`router.go`, `router.yaml`, `handler.go`)
- `middlewares/` — `api.Middleware` implementations
- `types/api/`, `types/entity/`, `types/model/`, `types/orm/`, `types/repo/` — request DTOs, domain entities, service interfaces, DB models, and repo interfaces, kept strictly separate
- `utils/` — stateless shared helpers

For the full category rules, type-separation example, server component file structure, and import organization, see [package-layout.md](package-layout.md).

## Common Mistakes

- `errors.New("msg")` does not compile — use `errors.Newf("msg")`. `New` takes functional options, not a message.
- Never return raw `err` — always `errors.Wrap(err)` or `errors.Wrapf(err, "context")`. Raw returns drop the stack trace.
- Never use `fmt.Errorf` or stdlib `errors.New` — the API server's error handler routes on `xhanio/errors` categories to set HTTP status.
- Don't place `.go` files at a `pkg/` category root — every category is a grouping folder; code lives in subdirectories (`pkg/services/user/`, not `pkg/services/foo.go`).
- A router's `prefix` MUST match the package's domain (e.g., `routers/user/` → `/users`), not an arbitrary path.
- Don't declare handlers as `func(c echo.Context) error` — use the project's `api.Context` (`<project>/pkg/types/api`). Handlers that take `echo.Context` compile and run, so nothing tells you it's wrong; you just lose `context.Context` (forcing `c.Request().Context()` at every service call) and the `Credential()`/`Session()`/`TraceID()` helpers.
- Don't hand-write the `Handlers()` map — each router's `router.go` returns `api.DiscoverHandlers(r)` (plus the debug log). Listing `map[string]any{"ListUsers": r.ListUsers}` by hand forces `echo.HandlerFunc` signatures (so you lose `api.Context`) and breaks on rename. Keep it in `router.go`; `handler.go` holds bodies only.
- Don't look for `Context` in framingo's `pkg/types/api` — it isn't there. `Context`/`DiscoverHandlers` are **project**-side (`example/pkg/types/api/api.go`); framingo's package (aliased `fapi`) only has `Router`, `Middleware`, `ErrorBody`, `ContextKey*`. Importing both unaliased is a compile error — alias the framework one `fapi`.
- Don't use the global Viper singleton — use the instance passed via `context.Context` (`confutil.FromContext(ctx)`).
- After `echo.Shutdown` is called, the same echo instance can't be reused — the framework's api server rebuilds it on `Init`, but custom services must do the same if they wrap net/http servers.

## Key Patterns

1. **Functional Options** - All services use `With*()` option functions for configuration
2. **Interface Composition** - Combine small interfaces (`Service` + `Initializable` + `Daemon`) as needed
3. **Context Propagation** - Config and transactions flow through `context.Context`
4. **Dependency Declaration** - Services declare dependencies via `Dependencies()`, manager resolves order
5. **Categorized Packages** - ALL code under `pkg/` must follow the category structure above
6. **Type Separation** - Each domain concept has `api/` (wire), `entity/` (domain), `orm/` (DB), plus `model/` (service interfaces) and `repo/` (repository interfaces)
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
