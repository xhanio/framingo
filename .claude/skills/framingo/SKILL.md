---
name: framingo
description: Build services and APIs using the Framingo Go framework. Use when creating services, registering them with the app manager, configuring HTTP servers/routers, working with the database service, pub/sub messaging, or implementing service lifecycle interfaces. Activate when user mentions framingo, service lifecycle, app manager, handler groups, or any framingo package imports.
compatibility: Requires Go 1.24+. Framework module is github.com/xhanio/framingo.
metadata:
  author: xhanio
  version: "1.0"
---

# Framingo - Service-Oriented Go Framework

Framingo is a modular, production-ready Go framework for building HTTP API applications with service lifecycle management, database integration, pub/sub messaging, and health monitoring.

## Architecture Overview

```
CLI / Config (cobra + viper)
    |
App Manager (lifecycle orchestration, dependency resolution, health monitoring)
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

### App Manager

Orchestrates all services. Located in `pkg/services/app`.

```go
import (
    "github.com/spf13/viper"
    "github.com/xhanio/framingo/pkg/services/app"
)

// Create manager with viper config
mgr := app.New(config,
    app.WithLogger(logger),
    app.WithMonitorInterval(30 * time.Second),
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

### Example Config YAML

This is the reference config structure for all framework services. Use this as a template when creating new applications:

```yaml
# Logging — used by log.New() in service.go
log:
  level: 0                    # 0=Debug, 1=Info, 2=Warn, 3=Error
  file: /var/log/myapp/app.log
  rotation:
    max_size: 100             # MB per log file
    max_backups: 3            # number of rotated files to keep
    max_age: 7                # days to retain old log files

# Database — used by db.New() options + dynamic config in db.Manager.Init()
db:
  type: postgres              # postgres | mysql | sqlite | clickhouse
  source:
    host: localhost
    port: 5432
    user: myapp
    password: secret
    dbname: myapp_db
  migration:
    dir: ./migrations         # path to migration SQL files
    version: 0                # target version (0 = latest)
  connection:
    max_open: 10              # max open connections
    max_idle: 5               # max idle connections
    max_lifetime: 1h          # connection max lifetime
    max_idle_time: 30m        # idle connection max lifetime
    exec_timeout: 30s         # query execution timeout

# API servers — iterated by m.config.GetStringMap("api") in service.go
# Each key becomes a named server instance via m.api.Add(name, ...)
api:
  http:                       # server name: "http"
    host: 0.0.0.0
    port: 8080
    prefix: /api/v1           # server endpoint path
    throttle:                 # optional: server-wide rate limiting
      rps: 100.0
      burst_size: 200
  admin:                      # server name: "admin"
    host: 0.0.0.0
    port: 9090
    prefix: /admin
  # HTTPS example with TLS
  # https:
  #   host: 0.0.0.0
  #   port: 443
  #   prefix: /api/v1
  #   cert: /path/to/server.crt
  #   key: /path/to/server.key

# TLS CA certificate (used when any api server has cert/key configured)
# ca:
#   cert: /path/to/ca.crt

# pprof profiling — optional, set port to enable
pprof:
  port: 6060                  # 0 = disabled

# Custom service config — read in service Init() via confutil.FromContext(ctx)
# example:
#   greeting: hello world!
```

**Notes**:
- `db.connection.*` keys are read dynamically during `db.Manager.Init(ctx)` via `confutil.FromContext(ctx)`, allowing values to change on service restart
- `api.*` is iterated as a string map — each top-level key under `api` becomes a named server instance
- TLS is enabled per-server when `api.<name>.cert` is set
- Throttle is enabled per-server when `api.<name>.throttle` is set
- Custom service config keys are accessed in `Init(ctx)` via `confutil.FromContext(ctx).GetString("myservice.key")`
- Pubsub and planner services are configured entirely via functional options, not YAML keys

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

Located in `pkg/services/api/server`. Built on Echo. The server uses a **declarative routing model**: routes are defined in YAML config, handler functions are provided by a Router implementation, and the server manager wires them together at registration time.

### Route Registration Flow

```
1. Server instances created    →  srvMgr.Add("http", ...)
2. Middlewares registered      →  srvMgr.RegisterMiddlewares(authMW, ...)
3. Routers registered         →  srvMgr.RegisterRouters(myRouter)
   a. Router.Config() called  →  returns embedded YAML ([]byte)
   b. YAML unmarshaled        →  produces HandlerGroup with Handlers
   c. Router.Handlers() called →  returns map[string]echo.HandlerFunc
   d. For each YAML handler:
      - Func name looked up in Handlers() map
      - HandlerKey generated: "serverName<METHOD>/prefix/path"
      - Handler func stored in manager.handlerFuncs[key]
   e. Server matched by HandlerGroup.Server field
   f. Echo group created at endpoint.Path + group.Prefix
   g. For each handler:
      - Handler-specific + group-level middlewares collected
      - Route registered: group.Add(method, path, handlerFunc, middlewares...)
      - Handler metadata stored on server for runtime lookup
```

### Creating a Server Manager

```go
import "github.com/xhanio/framingo/pkg/services/api/server"

srvMgr := server.New(
    server.WithLogger(logger),
    server.WithDebug(true),
)

// Step 1: Add named server instances (each gets its own echo.Echo)
srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/"))
srvMgr.Add("admin", server.WithEndpoint("0.0.0.0", 9090, "/admin"),
    server.WithThrottle(100, 200),
)

// Step 2: Register middlewares BEFORE routers (routers reference them by name)
srvMgr.RegisterMiddlewares(authMiddleware, corsMiddleware)

// Step 3: Register routers (triggers the full wiring flow above)
srvMgr.RegisterRouters(myRouter)
```

### Implementing a Router

A Router provides two things: a YAML config declaring routes, and a map of handler function implementations. The YAML `func` field is the lookup key into the `Handlers()` map.

```go
package user

import (
    _ "embed"
    "path"

    "github.com/labstack/echo/v4"

    "github.com/xhanio/framingo/pkg/types/api"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/log"
    "github.com/xhanio/framingo/pkg/utils/reflectutil"

    "myapp/pkg/services/user"
)

var _ api.Router = (*router)(nil) // compile-time interface check

//go:embed router.yaml
var config []byte

// Unexported struct — returns api.Router interface via factory
type router struct {
    name string
    log  log.Logger
    svc  user.Manager
}

func New(svc user.Manager, log log.Logger) api.Router {
    return &router{svc: svc, log: log}
}

func (r *router) Name() string {
    if r.name == "" {
        r.name = path.Join(reflectutil.Locate(r))
    }
    return r.name
}

func (r *router) Dependencies() []common.Service { return []common.Service{r.svc} }

// Config returns the embedded YAML that declares handler groups
func (r *router) Config() []byte { return config }

// Handlers returns func-name → implementation mapping
// Keys MUST match the "func" field in router.yaml
func (r *router) Handlers() map[string]echo.HandlerFunc {
    return map[string]echo.HandlerFunc{
        "ListUsers":  r.listUsers,
        "CreateUser": r.createUser,
        "GetUser":    r.getUser,
    }
}

func (r *router) listUsers(c echo.Context) error  { /* ... */ }
func (r *router) createUser(c echo.Context) error { /* ... */ }
func (r *router) getUser(c echo.Context) error    { /* ... */ }
```

### Router YAML Config Format (`router.yaml`)

The `server` field targets a named server instance created via `srvMgr.Add()`. The `func` field maps to keys in `Handlers()`.

**IMPORTANT**: The `prefix` MUST be a conventional REST resource path that matches the semantic meaning of the router package. The router package name and the prefix should describe the same domain concept:

| Router package | Prefix | Why |
|---|---|---|
| `routers/user/` | `/users` | Manages user resources |
| `routers/order/` | `/orders` | Manages order resources |
| `routers/auth/` | `/auth` | Authentication endpoints |
| `routers/example/` | `/example` | Example endpoints |

Do NOT use arbitrary or mismatched prefixes (e.g., a `routers/user/` package with prefix `/api/accounts`). The prefix is the public API contract — it must be intuitive and consistent with the package that owns it.

```yaml
server: http              # MUST match a server name from srvMgr.Add("http", ...)
prefix: /users            # MUST match the router package's domain (e.g., routers/user/ → /users)
middlewares: [auth]        # group-level middlewares (applied to ALL handlers in group)
handlers:
  - method: GET
    path: /
    func: ListUsers       # looked up in Router.Handlers()["ListUsers"]
  - method: POST
    path: /
    func: CreateUser      # looked up in Router.Handlers()["CreateUser"]
    middlewares: [validate]  # handler-specific middlewares (applied before group middlewares)
  - method: GET
    path: /:id
    func: GetUser
    throttle:              # per-handler rate limiting
      rps: 10
      burst_size: 20
```

### Handler Key Format

The server generates a unique key for each handler: `serverName<METHOD>/prefix/path`. For example, given the config above, the key for `ListUsers` is `http<GET>/users`. This key is used internally to store and look up handler functions and metadata.

### How Routes Map to Echo

The final Echo route path is: `server.endpoint.Path` + `group.Prefix` + `handler.Path`.

For a server added as `srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/"))` with the YAML above:
- `GET /users` → `ListUsers`
- `POST /users` → `CreateUser`
- `GET /users/:id` → `GetUser`

Root paths (`/`) are normalized by trimming the trailing slash, so both `/api/v1` and `/api/v1/` resolve to the same handler (via `RemoveTrailingSlash` pre-middleware).

### Router Interface (types)

```go
// pkg/types/api/model.go
type Router interface {
    common.Service
    Config() []byte                           // YAML config bytes (typically //go:embed)
    Handlers() map[string]echo.HandlerFunc    // func name → handler implementation
}

type Middleware interface {
    common.Service
    Func(echo.HandlerFunc) echo.HandlerFunc   // standard Echo middleware signature
}
```

### Middleware Resolution

Middlewares are registered by name via `RegisterMiddlewares()`. The YAML config references them by the name returned from `Middleware.Name()`. During route registration:

1. Handler-specific middlewares are resolved first (from `handler.middlewares`)
2. Group-level middlewares are resolved next (from `group.middlewares`)
3. If any referenced middleware name is not found, registration fails with `NotImplemented` error

Built-in server middlewares (applied to all routes automatically):
`Request → CORS (debug only) → Recover → Logger → Info → Error → Throttle → [Custom middlewares] → Handler → Response`

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

## Error Handling — `github.com/xhanio/errors`

**IMPORTANT**: All errors in framingo MUST use the `github.com/xhanio/errors` package. Do NOT use the standard `fmt.Errorf` or `errors.New` from the Go stdlib. The `xhanio/errors` package provides categorized errors with HTTP status codes, stack traces, and error wrapping — the API server's error handler relies on error categories to return correct HTTP responses.

### Import

```go
import "github.com/xhanio/errors"
```

### Creating Errors

**IMPORTANT**: Use `Newf()` to create errors with a message, NOT `New()`. The `New()` function takes functional `Option` arguments, not a message string. Using `New("some message")` will NOT compile.

```go
// Uncategorized errors (maps to 500 Internal Server Error)
errors.Newf("unsupported db type: %s", dbtype)

// Categorized errors (maps to specific HTTP status codes)
errors.BadRequest.Newf("invalid request: %v", err)
errors.NotFound.Newf("user %s not found", id)
errors.Unauthorized.Newf("invalid token")
errors.Conflict.Newf("resource %s already exists", name)
errors.NotImplemented.Newf("handler %s not found", funcName)

// Category as bare sentinel (no message needed) — this is the ONLY valid use of New()
return errors.NotImplemented.New()
```

### Wrapping Errors

**IMPORTANT**: ALWAYS use `errors.Wrap(err)` or `errors.Wrapf(err, msg, ...)` when returning errors from called functions. NEVER return a raw `err` directly — this loses the stack trace. Every error must be wrapped to maintain the full call chain for debugging.

```go
// CORRECT — wrap to maintain error stack
if err := m.initConfig(); err != nil {
    return errors.Wrap(err)
}

// CORRECT — wrap with additional context message when helpful
if err := m.db.FromContext(ctx).Create(record).Error; err != nil {
    return errors.Wrapf(err, "failed to create user %s", name)
}

// CORRECT — wrap with a category (overrides the wrapped error's category)
return errors.BadRequest.Wrap(err)
return errors.DBFailed.Wrapf(err, "query failed for user %s", id)

// WRONG — never return raw err, stack trace is lost
if err := doSomething(); err != nil {
    return err  // ❌ DO NOT DO THIS
}
```

### Combining Multiple Errors

```go
// Combine multiple errors (uses uber/multierr under the hood)
return errors.Combine(errs...)
```

### Checking Errors

```go
// Check if error belongs to a category
if errors.Is(err, errors.NotFound) { /* ... */ }

// Check if error wraps a specific cause
if errors.Has(err, ErrConnection) { /* ... */ }
```

### Available Error Categories

| Category | HTTP Status | Use For |
|---|---|---|
| `BadRequest` | 400 | Invalid input, malformed requests |
| `InvalidArgument` | 400 | Invalid function arguments |
| `Unauthorized` | 401 | Missing or invalid authentication |
| `Forbidden` | 403 | Authenticated but not authorized |
| `PermissionDenied` | 403 | Insufficient permissions |
| `NotFound` | 404 | Resource not found |
| `DeadlineExceeded` | 408 | Timeout |
| `Conflict` | 409 | Resource already exists, concurrent modification |
| `AlreadyExist` | 409 | Duplicate resource |
| `TooManyRequests` | 429 | Rate limit exceeded |
| `Cancaled` | 499 | Operation cancelled |
| `Internal` | 500 | Unexpected internal errors |
| `NotImplemented` | 501 | Unimplemented functionality |
| `Unavailable` | 503 | Service unavailable |
| `ResourceExhausted` | 503 | Out of resources |
| `DBFailed` | 500 | Database operation failures |

### Custom Categories

```go
var ErrPaymentFailed = errors.NewCategory("PaymentFailed", 402)

// Then use like built-in categories:
return ErrPaymentFailed.Newf("charge declined for order %s", orderID)
```

## Package Organization

**IMPORTANT**: All application packages MUST follow the categorized directory structure under `pkg/`. This is a strict convention — do NOT place code outside these categories or flatten the hierarchy.

### Required `pkg/` Structure

```
pkg/
├── components/          # Top-level application components (wires everything together)
│   ├── cmd/             # CLI commands (cobra commands)
│   │   └── myapp/       # root.go, daemon.go, common.go
│   └── server/          # Server components (creates services, registers, starts)
│       └── myapp/       # manager.go, config.go, service.go, signal.go, api.go
├── services/            # Business logic services (implement Service/Daemon interfaces)
│   └── user/            # manager.go, model.go, option.go
├── routers/             # HTTP route handlers (implement api.Router)
│   └── user/            # router.go, router.yaml, handler.go
├── middlewares/          # HTTP middlewares (implement api.Middleware)
│   └── auth/            # middleware.go
├── types/               # Pure data types — NO logic, NO imports from services
│   ├── api/             # Request/response structs (JSON + validation tags)
│   ├── entity/          # Business domain models (JSON tags only)
│   └── orm/             # Database models (GORM tags only)
└── utils/               # Shared utility packages
    └── infra/           # Infrastructure helpers
```

**IMPORTANT**: Every `pkg/` category directory is a grouping folder only — NEVER place Go source files directly in a category root. Each category MUST contain subdirectories that hold the actual code. For example, `types/` contains `api/`, `entity/`, `orm/`; `services/` contains `user/`, etc. The category root itself has no `.go` files.

### Category Rules

| Category | Purpose | Key Rule |
|---|---|---|
| `components/` | Application wiring — creates services, registers them with app manager, handles config and signals | Only place that knows about ALL services; the "main" of each deployable |
| `services/` | Business logic — each service is a self-contained unit with its own `Manager` interface | Must declare dependencies via `Dependencies()`, never import other services directly |
| `routers/` | HTTP handlers — each router owns a `router.yaml` + `Handlers()` map | Delegates business logic to services, never contains domain logic itself |
| `middlewares/` | Request processing — each middleware implements `api.Middleware` | Stateless request/response transformations only |
| `types/api/` | API request/response structs | Tags: `json`, `form`, `query`, `validate`. NO gorm tags |
| `types/entity/` | Pure business domain models | Tags: `json` only. Returned from services to callers |
| `types/orm/` | Database table models | Tags: `gorm` only. Must implement `TableName()`. Never exposed outside services |
| `utils/` | Shared helpers | Must be stateless, no service dependencies |

### Type Separation Example

The same domain concept has THREE separate type representations:

```go
// types/api/user.go — request validation
type CreateUserRequest struct {
    Name  string `json:"name" form:"name" validate:"required"`
    Email string `json:"email" form:"email" validate:"required,email"`
}

// types/entity/user.go — clean business model
type User struct {
    ID        int64     `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

// types/orm/user.go — database model
type User struct {
    ID        int64     `gorm:"primaryKey"`
    Name      string    `gorm:"type:varchar(255);not null"`
    Email     string    `gorm:"type:varchar(255);uniqueIndex;not null"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
func (User) TableName() string { return "users" }
```

**Data flows**: Router receives `api.CreateUserRequest` → calls service → service uses `orm.User` for DB → returns `entity.User` to router → router sends JSON response.

### Server Component — Application Daemon

**IMPORTANT**: All server implementations MUST follow the file structure under `example/pkg/components/server/example/`. This is the standard pattern for creating the application daemon. Each file has a specific responsibility:

```
components/server/myapp/
├── model.go    # Server interface definition (Named + Daemon + Initializable + Debuggable)
├── manager.go  # Main struct, New(), Init(), Start(), Stop(), Info() — orchestrates everything
├── config.go   # Viper config creation (newConfig) and loading (initConfig)
├── service.go  # initServices() — creates ALL service instances in layered order
├── api.go      # initAPI() — registers middlewares and routers with the API server
└── signal.go   # listenSignals() — OS signal handling (SIGINT, SIGTERM, SIGUSR1, SIGUSR2)
```

#### `model.go` — Server Interface

```go
type Server interface {
    common.Named
    common.Daemon        // Start(ctx) / Stop(wait)
    common.Initializable // Init(ctx)
    common.Debuggable    // Info(w, debug)
}
```

#### `manager.go` — Orchestrator

The manager holds all service references and implements the `Server` interface. `Init()` calls `initConfig()` → `initServices()` → registers services with app manager → `TopoSort()` → `services.Init()` → `initAPI()`. `Start()` starts all services and blocks on `<-ctx.Done()`.

```go
type manager struct {
    name   string
    config *viper.Viper
    log    log.Logger

    // utility services
    db db.Manager

    // system services
    bus pubsub.Manager

    // business services
    userSvc user.Manager

    // api services
    mws []api.Middleware
    api server.Manager

    // service controller
    services app.Manager
    cancel   context.CancelFunc
}

func New(configPath string) Server {
    return &manager{config: newConfig(configPath)}
}
```

#### `service.go` — Service Creation (Layered Order)

Services MUST be created in this layered order:

```go
func (m *manager) initServices() error {
    // 1. Logger — first, everything depends on it
    m.log = log.New(...)

    // 2. Database
    m.db = db.New(db.WithType(...), db.WithDataSource(...), db.WithLogger(m.log))

    // 3. App manager (service controller)
    m.services = app.New(m.config, app.WithLogger(m.log))

    // 4. System services (pubsub, etc.)
    m.bus = pubsub.New(driver.NewMemory(m.log), pubsub.WithLogger(m.log))

    // 5. Business services
    m.userSvc = user.New(m.db, user.WithLogger(m.log))

    // 6. API server (created last, started last)
    m.api = server.New(server.WithLogger(m.log))
    // Add server instances from config
    servers := m.config.GetStringMap("api")
    for name := range servers {
        m.api.Add(name, server.WithEndpoint(...))
    }
    return nil
}
```

#### `api.go` — Middleware and Router Registration

```go
func (m *manager) initAPI() error {
    middlewares := []api.Middleware{
        authmw.New(),
    }
    routers := []api.Router{
        userRouter.New(m.userSvc, m.log),
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

#### `signal.go` — OS Signal Handling

```go
func (m *manager) listenSignals(ctx context.Context) {
    // SIGINT/SIGTERM  → graceful shutdown (services.Stop + cancel)
    // SIGUSR1         → dump service info to stdout (m.Info(os.Stdout, true))
    // SIGUSR2         → dump goroutine stack trace
}
```

#### Registration Order in `manager.go Init()`

```go
func (m *manager) Init(ctx context.Context) error {
    m.initConfig()
    m.initServices()

    // Register services in dependency layers
    m.services.Register(m.db)                    // basic services
    m.services.Register(m.bus, m.userSvc)        // system + business services
    m.services.TopoSort()                        // resolve dependency order
    m.services.Register(m.api)                   // API registered AFTER sort to ensure it starts last

    // Subscribe all services to pubsub bus
    for _, svc := range m.services.Services() {
        m.bus.Subscribe(svc, "/")
    }

    m.services.Init(ctx)                         // init all services in dependency order
    m.initAPI()                                  // wire routes after services are initialized
    return nil
}
```

## Import Organization

**IMPORTANT**: All Go imports MUST be organized into exactly three groups, separated by blank lines:

1. **Go standard library** packages
2. **Third-party** packages
3. **Project** packages (current module and `github.com/xhanio/*`)

```go
import (
    // Group 1: Go standard library
    "context"
    "database/sql"
    "fmt"
    "time"

    // Group 2: Third-party packages
    "github.com/labstack/echo/v4"
    "github.com/spf13/viper"
    "gorm.io/gorm"

    // Group 3: Project packages (xhanio/* and current module)
    "github.com/xhanio/errors"
    "github.com/xhanio/framingo/pkg/services/db"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/log"
)
```

Never mix groups. Never use more than three groups. Each group is alphabetically sorted.

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
