# Services

This directory contains all service implementations for the example application. Services are organized into three layers, each with a single, well-defined responsibility.

## Layered Architecture

```
example/         business logic (HelloWorld)
system/*         business logic for cross-cutting concerns
                 (auth, user, role, organization, certificate)
        ↓
repository/      data access; gorm-backed implementations
                 of per-domain repo interfaces
        ↓
framework db.Manager (pkg/services/db) → gorm.DB → driver
```

Dependency rules:

- **`repository/`** depends only on the framework `db.Manager`. No business logic, no validation, no HTTP concerns — just queries, transactions, and gorm error mapping.
- **`system/*`** depends on `repository.Repository` (and on other system services where noted). Business code never calls `m.db.FromContext(ctx)` or touches gorm directly.
- **`example/`** depends on `repository.Repository`. Application-level business services may additionally depend on `system/*` services as needed.

## Repository Layer ([repository/](repository/))

A single `Repository` interface ([model.go](repository/model.go)) embeds per-domain interfaces declared under [`pkg/types/repo/`](../types/repo/):

| File | Domain |
|------|--------|
| [user.go](repository/user.go) | Users + contacts |
| [role.go](repository/role.go) | Roles + permissions |
| [organization.go](repository/organization.go) | Organizations / tenants |
| [certificate.go](repository/certificate.go) | Certificates |
| [example.go](repository/example.go) | Helloworld (example) |

`New(db model.Database, opts ...Option) Repository` — depends on the framework `db.Manager`. Also exposes `Transaction(ctx, fn, opts...)` for service-side transaction boundaries.

## System Services ([system/](system/))

Cross-cutting business services. Each follows the same per-service file pattern:

```
<service>/
├── manager.go    # struct, New(...), Name(), Dependencies()
├── lifecycle.go  # Init / Start / Stop / message handlers
├── business.go   # interface methods — validation + repo calls + entity conversion
├── model.go      # Manager interface (composes model.* + common.*)
├── option.go     # functional options (WithLogger, ...)
└── util.go       # internal helpers (some services)
```

The `Manager` interface for each service is composed of a business interface from [`pkg/types/model/`](../types/model/) plus the relevant `common.*` lifecycle interfaces.

| Service | Purpose | `New(...)` dependencies |
|---------|---------|-------------------------|
| [auth/](system/auth/) | Login / sessions, user + LDAP + API token authentication, session lookup and refresh | `model.UserAuthN`, `model.LDAPAuthN`, `model.APITokenAuthN` (latter two optional) |
| [user/](system/user/) | User CRUD, password management, contacts; also implements `model.UserAuthN` consumed by `auth` | `repository.Repository` |
| [role/](system/role/) | RBAC roles and permissions | `repository.Repository` |
| [organization/](system/organization/) | Organization / tenant model | `repository.Repository` |
| [certificate/](system/certificate/) | Certificate issuance and management | `repository.Repository` |

`user/` and `role/` ship with `manager_test.go` covering the business layer against a mock repository.

## Example Service ([example/](example/))

The `example` service demonstrates the full pattern end-to-end on a single `Helloworld` entity. It uses the same `manager.go` / `lifecycle.go` / `business.go` / `model.go` / `option.go` layout as the system services and delegates persistence to `repository.Repository`.

`New(repo repository.Repository, opts ...Option) Manager`

See [example/README.md](example/README.md) for the full walkthrough — service construction, lifecycle, option patterns, entity vs. ORM type separation, and the end-to-end repository pattern (service-side validation + repo-side gorm error mapping + service-spanning transactions).

## Conventions

Business code (`business.go` in `system/*` and `example/`):

- Validate inputs, call the repo, convert ORM → entity.
- No `m.db.FromContext`, no `tx.Transaction`, no gorm types.
- The one exception: wrap multi-repo atomic operations with `m.db.Transaction(ctx, func(_ctx context.Context) error { ... })`.

Repository code (`repository/*.go`):

- Public methods take `context.Context`; private helpers take `tx *gorm.DB`.
- Wrap multi-step writes in `tx.Transaction(...)`.
- Map gorm errors: `gorm.ErrRecordNotFound` → `errors.NotFound`, `gorm.ErrDuplicatedKey` → `errors.AlreadyExist`, default → `errors.DBFailed`.
- Always return errors via `errors.Wrap` / `errors.Wrapf`.

## See Also

- [Framework service interfaces](../../../pkg/types/common/service.go)
- [Business interfaces](../types/model/) — `model.User`, `model.Role`, `model.Auth`, ...
- [Repository interfaces](../types/repo/) — per-domain contracts
- [Entity / ORM type separation](../types/README.md)
