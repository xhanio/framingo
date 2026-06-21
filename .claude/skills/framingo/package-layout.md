# Framingo Package Layout Reference

Full reference for the required `pkg/` directory structure, category rules, type separation, the server component file layout, and import organization conventions.

## Package Organization

**IMPORTANT**: All application packages MUST follow the categorized directory structure under `pkg/`. This is a strict convention — do NOT place code outside these categories or flatten the hierarchy.

## Required `pkg/` Structure

```
pkg/
├── components/          # Top-level application components (wires everything together)
│   ├── client/          # Go client SDK — typed methods over HTTP for external callers
│   │   └── myapp/       # client.go, auth.go, model.go, option.go (+ per-domain files)
│   ├── cmd/             # Cobra command trees — one subdir per deployable binary
│   │   ├── app/         # Daemon binary entrypoint: root.go, daemon.go, common.go
│   │   └── cli/         # CLI binary entrypoint: root.go, common.go (+ per-subcommand files)
│   └── server/          # Application daemon — owns the supervisor, wires all services
│       └── myapp/       # model.go, manager.go, lifecycle.go, config.go, service.go, api.go, signal.go
├── services/            # Business logic services (implement Service/Daemon interfaces)
│   └── user/            # manager.go, lifecycle.go, business.go, model.go, option.go
├── routers/             # HTTP route handlers (implement api.Router)
│   └── user/            # router.go, router.yaml, handler.go
├── middlewares/         # HTTP middlewares (implement api.Middleware)
│   └── auth/            # middleware.go
├── types/               # Pure data types — NO logic, NO imports from services
│   ├── api/             # Request/response structs (json + form + query + validate tags)
│   ├── entity/          # Business domain models (json tags only)
│   ├── model/           # Service interfaces — one file per domain (e.g., user.go declares model.User)
│   ├── orm/             # Database table models (gorm tags only; must implement TableName())
│   └── repo/            # Repository interfaces — one file per domain (data-access contracts)
└── utils/               # Shared utility packages
    └── infra/           # Infrastructure helpers
```

**IMPORTANT**: Every `pkg/` category directory is a grouping folder only — NEVER place Go source files directly in a category root. Each category MUST contain subdirectories that hold the actual code. For example, `types/` contains `api/`, `entity/`, `orm/`; `services/` contains `user/`, etc. The category root itself has no `.go` files.

## Category Rules

| Category | Purpose | Key Rule |
|---|---|---|
| `components/cmd/` | Cobra command trees — one subdir per deployable binary (e.g., `app/` for the daemon, `cli/` for the client) | Subdir name maps 1:1 to `build/binary/<name>/main.go`; no business logic — only flag parsing and delegation |
| `components/server/` | Application daemon — owns the supervisor, wires all services, handles signals | Only place that knows about ALL services; one file per concern (see Server Component below) |
| `components/client/` | Go client SDK exposing typed methods over HTTP for the daemon's API | Consumed by `components/cmd/cli/` and external callers; depends only on `types/api/` and `types/entity/`, never on services |
| `services/` | Business logic — each service is a self-contained unit with its own `Manager` interface | Must declare dependencies via `Dependencies()`, never import other services directly; implementation is unexported, exposed via interface from `types/model/` |
| `routers/` | HTTP handlers — each router owns a `router.yaml` + `Handlers()` map | Delegates business logic to services, never contains domain logic itself |
| `middlewares/` | Request processing — each middleware implements `api.Middleware` | Stateless request/response transformations only |
| `types/api/` | API request/response structs | Tags: `json`, `form`, `query`, `validate`. NO gorm tags |
| `types/entity/` | Pure business domain models | Tags: `json` only. Returned from services to callers |
| `types/model/` | Service interfaces — one file per domain (`user.go` declares `model.User`) | Imported by routers and other services so the implementation can stay package-private; the public contract layer |
| `types/orm/` | Database table models | Tags: `gorm` only. Must implement `TableName()`. Never exposed outside services |
| `types/repo/` | Repository interfaces — one file per domain (data-access contracts) | Implemented by `services/repository/`; services depend on the interface, never on a concrete repo struct |
| `utils/` | Shared helpers | Must be stateless, no service dependencies |

## Type Separation Example

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

## Server Component — Application Daemon

**IMPORTANT**: All server implementations MUST follow the file structure under `example/pkg/components/server/example/`. This is the standard pattern for creating the application daemon. Each file has a specific responsibility:

```
components/server/myapp/
├── model.go     # Server interface definition (Named + Daemon + Initializable + Debuggable)
├── manager.go   # Main struct, New(), Name() — struct fields and construction only
├── lifecycle.go # Init(), Start(), Stop(), Info() — orchestrates everything in order
├── config.go    # Viper config creation (newConfig) and loading (initConfig)
├── service.go   # initServices() — creates ALL service instances in layered order
├── api.go       # initAPI() — registers middlewares and routers with the API server
└── signal.go    # listenSignals() — OS signal handling (SIGINT, SIGTERM, SIGUSR1, SIGUSR2)
```

### `model.go` — Server Interface

```go
type Server interface {
    common.Named
    common.Daemon        // Start(ctx) / Stop(wait)
    common.Initializable // Init(ctx)
    common.Debuggable    // Info(w, debug)
}
```

### `manager.go` — Struct and Construction

`manager.go` holds the struct definition and the `New()`/`Name()` methods only. Lifecycle methods live in `lifecycle.go`.

```go
type manager struct {
    name   string
    config *viper.Viper
    log    log.Logger

    // infra services
    db         db.Manager
    pubsub     pubsub.Manager
    messagebus messagebus.Manager

    // business services
    userSvc user.Manager

    // api services
    api server.Manager

    // service controller
    services supervisor.Manager
    ctx      context.Context
    cancel   context.CancelFunc
}

func New(configPath string) Server {
    return &manager{config: newConfig(configPath)}
}
```

### `lifecycle.go` — Orchestration

Implements `Init`, `Start`, `Stop`, `Info`. `Init()` calls `initConfig()` → `initServices()` → registers services with supervisor in dependency layers → `TopoSort()` → registers `m.api` AFTER the sort so it starts last → registers all services with the messagebus → `services.Init()` → `initAPI()`. `Start()` starts all services and blocks on `<-ctx.Done()`.

### `service.go` — Service Creation (Layered Order)

Services MUST be created in this layered order:

```go
func (m *manager) initServices() error {
    // 1. Logger — first, everything depends on it
    m.log = log.New(...)

    // 2. Supervisor (service controller)
    m.services = supervisor.New(m.config, supervisor.WithLogger(m.log))

    // 3. Infra services: database, pubsub, messagebus
    m.db = db.New(db.WithType(...), db.WithDataSource(...), db.WithLogger(m.log))
    m.pubsub = pubsub.New(driver.NewMemory(m.log), pubsub.WithLogger(m.log))
    m.messagebus = messagebus.New(m.pubsub, messagebus.WithLogger(m.log))

    // 4. Business services
    m.userSvc = user.New(m.db, user.WithLogger(m.log))

    // 5. API server (created last, started last)
    m.api = server.New(server.WithLogger(m.log))
    servers := m.config.GetStringMap("api")
    for name := range servers {
        m.api.Add(name, server.WithEndpoint(...))
    }
    return nil
}
```

### `api.go` — Middleware and Router Registration

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

### `signal.go` — OS Signal Handling

```go
func (m *manager) listenSignals(ctx context.Context) {
    // SIGINT/SIGTERM  → graceful shutdown (services.Stop + cancel)
    // SIGUSR1         → dump service info to stdout (m.Info(os.Stdout, true))
    // SIGUSR2         → dump goroutine stack trace
}
```

### Registration Order in `lifecycle.go Init()`

```go
func (m *manager) Init(ctx context.Context) error {
    m.initConfig()
    m.initServices()

    // Register in dependency layers
    m.services.Register(m.db)                                // basic infra
    m.services.Register(m.pubsub, m.messagebus, m.userSvc)   // system + business
    m.services.TopoSort()                                    // resolve dependency order
    m.services.Register(m.api)                               // API registered AFTER sort to ensure it starts last

    // Register all services with the messagebus. Services that don't implement
    // MessageHandler / RawMessageHandler are skipped automatically.
    for _, svc := range m.services.Services() {
        m.messagebus.Register(svc)
    }

    m.services.Init(ctx)                                     // init all services in dependency order
    m.initAPI()                                              // wire routes after services are initialized
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
