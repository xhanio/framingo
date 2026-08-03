# Project Types — The `pkg/types/` Categories

Every domain concept splits across the project's `types/` subdirectories,
kept strictly separate. All five are part of the layout
([layout.md](layout.md)); none is optional.

| Directory | Holds | Consumed by |
|---|---|---|
| `types/api/` | Wire types: request/response DTOs; in the project also `Context` + `DiscoverHandlers` | Routers, clients |
| `types/entity/` | Domain entities (business data, no ORM tags) | Services, routers |
| `types/model/` | Service business interfaces (`model.Order` — methods + `common.Service`, no lifecycle) | Routers and other services — see [services.md](services.md) |
| `types/orm/` | DB models implementing the framework's ORM base interfaces ([types.md](../pkgs/types.md)) | Repositories, [db.md](../pkgs/db.md) |
| `types/repo/` | Repository interfaces | Services |

The framework's own `pkg/types/` — the `common` interfaces, `fapi`, ORM base
types, context keys — is the pkgs half: [types.md](../pkgs/types.md).

## The two `api` packages

- `fapi` — **framingo's** `pkg/types/api`: `Router`, `Middleware`, `RequestInfo`, `CORSConfig`, `Endpoint`, TLS types, `ErrorBody`, `WrapError`, `ContextKey*`. No `Context`.
- `api` — **your project's** `pkg/types/api`: `Context`, `DiscoverHandlers`, DTOs, and middleware config block shapes (the example keeps `ThrottleConfig` there).

Import both, framework one aliased `fapi` — full treatment in [routers.md](routers.md).
