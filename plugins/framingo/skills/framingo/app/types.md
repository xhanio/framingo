# Project Types — The `pkg/types/` Categories

Every domain concept splits across the project's `types/` subdirectories,
kept strictly separate. Five are core — the layout ([layout.md](layout.md))
and the scaffold require them — and the list grows with the app: the example
adds three more.

| Directory | Holds | Consumed by |
|---|---|---|
| `types/api/` | Wire types: request/response DTOs, middleware config blocks (`ThrottleConfig` in `middleware.go`), plus `Context` + `DiscoverHandlers` in `api.go` | Routers, clients |
| `types/entity/` | Domain entities (business data, no ORM tags) | Services, routers |
| `types/model/` | Service business interfaces (`model.Example` — methods + `common.Service`, no lifecycle) | Routers and other services — see [services.md](services.md) |
| `types/orm/` | DB models implementing the framework's ORM base interfaces ([types.md](../pkgs/types.md)) | `services/repository/`, [db.md](../pkgs/db.md) |
| `types/repo/` | Repository interfaces, one file per domain — all implemented by `services/repository/` | Services |

The example's grown categories:

| Directory | Holds |
|---|---|
| `types/message/` | Typed message-bus payloads, each implementing `Kind() string` — `message.Example{From, To, Message}` is kind `"example_event"` |
| `types/preset/` | Seeded constants and defaults: auth sources, session TTLs, preset roles and organizations |
| `types/rbac/` | Permission and feature name constants — shared by the authz middleware, the role service, and `permission:` entries in router.yaml |

The framework's own `pkg/types/` — the `common` interfaces, `fapi`, ORM base
types, context keys — is the pkgs half: [types.md](../pkgs/types.md).

## One Concept, One Type per Layer

The example's helloworld flow shows the separation at its smallest — four
types, four jobs, no tags crossing layers:

```go
// types/api/example.go — wire (validated on the way in)
type HelloWorldCreateRequest struct {
	Message string `json:"message" form:"message" query:"message" validate:"required"`
}

// types/entity/example.go — domain (what services return)
type HelloWorld struct {
	ID        int64     `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// types/orm/example.go — persistence (gorm only, owns its table name)
type HelloWorld struct {
	ID        int64     `gorm:"primaryKey"`
	Message   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (HelloWorld) TableName() string { return "helloworld_messages" }

// types/repo/example.go — the data-access contract
type HelloWorld interface {
	CreateHelloWorld(ctx context.Context, message string) (*orm.HelloWorld, error)
}
```

**The flow**: the router binds `api.HelloWorldCreateRequest` → calls
`model.Example` → the service calls `repo.HelloWorld` (through
`repository.Repository`), which persists `orm.HelloWorld` → the service maps
it to `entity.HelloWorld` and returns that.

## The two `api` packages

- `fapi` — **framingo's** `pkg/types/api`: `Router`, `Middleware`, `RequestInfo`, `CORSConfig`, `Endpoint`, TLS types, `ErrorBody`, `WrapError`, `ContextKey*`. No `Context`.
- `api` — **your project's** `pkg/types/api`: `Context`, `DiscoverHandlers`, DTOs, and middleware config block shapes (the example keeps `ThrottleConfig` there).

Import both, framework one aliased `fapi` — full treatment in [routers.md](routers.md).
