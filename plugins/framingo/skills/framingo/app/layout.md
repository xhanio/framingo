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
│   ├── api/             # Request/response structs (json + form + query + validate tags), plus the project's api.Context wrapper in api.go
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
| `components/cmd/` | Cobra command trees — one subdir per deployable binary (e.g., `app/` for the daemon, `cli/` for the client) | One subdir per binary under `build/binary/`, but the names need not match: the example pairs `cmd/app` → `build/binary/exampleapp` and `cmd/cli` → `build/binary/examplecli`. No business logic — only flag parsing and delegation |
| `components/server/` | Application daemon — owns the supervisor, wires all services, handles signals | Only place that knows about ALL services; one file per concern (see Server Component below) |
| `components/client/` | Go client SDK exposing typed methods over HTTP for the daemon's API | Consumed by `components/cmd/cli/` and external callers; depends only on `types/api/` and `types/entity/`, never on services |
| `services/` | Business logic — each service is a self-contained unit with its own `Manager` interface | Must declare dependencies via `Dependencies()`, never import other services directly; implementation is unexported, exposed via interface from `types/model/` |
| `routers/` | HTTP handlers — each router owns a `router.yaml` + a `Handlers()` that returns `api.DiscoverHandlers(r)` with a debug log | Split per package into `router.go` (wiring: factory, `Name`/`Dependencies`/`Config`/`Handlers`) + `handler.go` (handler bodies, taking the project `api.Context`). Delegates business logic to services, never contains domain logic itself |
| `middlewares/` | Request processing — each middleware implements `api.Middleware` | Stateless request/response transformations only |
| `types/api/` | API request/response structs | Tags: `json`, `form`, `query`, `validate`. NO gorm tags |
| `types/entity/` | Pure business domain models | Tags: `json` only. Returned from services to callers |
| `types/model/` | Service **business** interfaces — one file per domain (`user.go` declares `model.User`) | Imported by routers and other services so the implementation can stay package-private; the public contract layer. Pairs with the service's own `Manager` — see below |
| `types/orm/` | Database table models | Tags: `gorm` only. Must implement `TableName()`. Never exposed outside services |
| `types/repo/` | Repository interfaces — one file per domain (data-access contracts) | Implemented by `services/repository/`; services depend on the interface, never on a concrete repo struct |
| `utils/` | Shared helpers | Must be stateless, no service dependencies |

### `types/model/` vs the service's own `Manager` — both, not either

Every service declares its interface in two places. They are not duplicates:

| File | Declares | Consumed by |
|---|---|---|
| `types/model/user.go` | `model.User` — business methods + `common.Service`, **no lifecycle** | Routers, other services |
| `services/user/model.go` | `user.Manager` = `model.User` + the lifecycle interfaces it implements | Only `components/server/` wiring |

A router's constructor takes `model.User`, never `user.Manager` — that's what stops the router from seeing (or calling) the service's `Start`/`Stop`. A router importing `pkg/services/...` has skipped the split.

Templates for both files: [`_templates/types-model-order.go`](../_templates/types-model-order.go) and [`_templates/services-order-model.go`](../_templates/services-order-model.go).

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

All server implementations MUST follow the fixed file-per-responsibility
structure (`model.go` / `manager.go` / `lifecycle.go` / `config.go` /
`service.go` / `api.go` / `signal.go`), specified in full — layered service
creation, registration order, signals — in
[components-server.md](components-server.md). The reference implementation is
[`example/pkg/components/server/example/`](https://github.com/xhanio/framingo/tree/main/example/pkg/components/server/example).

## Import Organization

**IMPORTANT**: All Go imports MUST be organized into exactly three groups, separated by blank lines:

1. **Go standard library**
2. **Everything external to your module** — third-party libraries *and* `github.com/xhanio/framingo/*` / `github.com/xhanio/errors`
3. **Your own module** only

The framework is a dependency like any other, so it goes in group 2, **not** with your own packages. Group 3 holds exclusively the current module.

```go
import (
    // Group 1: Go standard library
    _ "embed"
    "context"
    "path"

    // Group 2: third-party + framingo (external to this module)
    "github.com/labstack/echo/v4"
    "github.com/xhanio/errors"
    fapi "github.com/xhanio/framingo/pkg/types/api"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/log"
    "gorm.io/gorm"

    // Group 3: this module
    "myapp/pkg/services/order"
    "myapp/pkg/types/api"
)
```

Never mix groups. Never use more than three groups. Each group is alphabetically sorted.

This is what every file under [`example/pkg/`](https://github.com/xhanio/framingo/tree/main/example/pkg) does, and what the files in [`_templates/`](../_templates/) do — when in doubt, copy their grouping.
