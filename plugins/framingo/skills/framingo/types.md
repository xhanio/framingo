# Types — The `pkg/types/` Categories

Every domain concept splits across the `types/` subdirectories, kept strictly
separate. All five are part of the layout ([package-layout.md](package-layout.md));
none is optional.

| Directory | Holds | Consumed by |
|---|---|---|
| `types/api/` | Wire types: request/response DTOs; in the project also `Context` + `DiscoverHandlers` | Routers, clients |
| `types/entity/` | Domain entities (business data, no ORM tags) | Services, routers |
| `types/model/` | Service business interfaces (`model.Order` — methods + `common.Service`, no lifecycle) | Routers and other services — see [services.md](services.md) |
| `types/orm/` | DB models + the ORM base interfaces below | Repositories, [database.md](database.md) |
| `types/repo/` | Repository interfaces | Services |

The framework's own `pkg/types/` adds `common` (service/lifecycle/message
interfaces, context keys) and its `api` package (`fapi`).

## The two `api` packages

- `fapi` — **framingo's** `pkg/types/api`: `Router`, `Middleware`, `RequestInfo`, `CORSConfig`, `Endpoint`, TLS types, `ErrorBody`, `WrapError`, `ContextKey*`. No `Context`.
- `api` — **your project's** `pkg/types/api`: `Context`, `DiscoverHandlers`, DTOs, and middleware config block shapes (the example keeps `ThrottleConfig` there).

Import both, framework one aliased `fapi` — full treatment in [routers.md](routers.md).

## ORM Base Types

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

## Context Keys

Defined in `pkg/types/common/context.go`:
- `_config` - Viper config instance
- `_tx` - Database transaction (`*gorm.DB`)
- `_db` - Database reference
- `_logger` - Logger instance
- `_trace` - Trace context
- `_credential`, `_session`, `_namespace` - Auth context
- `_api_request_info`, `_api_response_info`, `_api_error` - API context

## Message Interfaces

`Message`, `MessageHandler`, `RawMessageHandler`, `MessageSender`,
`RawMessageSender` live in `pkg/types/common` too — signatures and usage in
[pubsub.md](pubsub.md).
