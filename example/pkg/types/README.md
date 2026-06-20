# Types Package

This package contains the type definitions used throughout the application, organized by concern: API shapes, business entities, ORM rows, and repository contracts.

## Directory Structure

### `api/`
Request/response shapes for HTTP endpoints, plus the request-scoped context bridge.

- Files: `auth.go`, `certificate.go`, `context.go`, `dashboard.go`, `example.go`, `user.go`.
- Include validation + binding tags: `json`, `form`, `query`, `validate`.
- Never include GORM tags.
- API-shape types belong here, **not** in `entity/`.

```go
type CreateHelloWorldMessage struct {
    Message string `json:"message" form:"message" query:"message" validate:"required"`
}
```

#### `context.go` — echo/context bridge

`api.Context` embeds both `echo.Context` and `context.Context`, so a single value flows through handlers, services, and the repo without manual extraction of `c.Request().Context()`.

- The unexported `ctx` struct wraps `echo.Context` and implements `Deadline/Done/Err` by delegating to the underlying request context.
- `Value(key)` translates string keys to `c.Get(string)`, so anything middleware sets via `c.Set("_credential", ...)` is reachable from a downstream `context.Context.Value(...)` call.
- Typed accessors: `Credential()`, `Session()`, `TraceID()` — each returns `(value, ok)` against the keys defined in `framingo/pkg/types/api`.
- Bind helpers: `BindQuery()`, `BindPath()`, `BindForm()` return echo `ValueBinder`s; `BindAny(i)` delegates to `c.Bind`.
- `WrapHandler(HandlerFunc) echo.HandlerFunc` adapts handlers written against `api.Context` so they can be registered on the echo router.

### `entity/`
Pure business-domain types — independent of storage and transport.

- Files: `auth.go`, `backup.go`, `certificate.go`, `example.go`, `organization.go`, `role.go`, `system.go`, `user.go`.
- JSON tags only, snake_case (`json:"first_name"`).
- No GORM, no validation tags.
- Used by services and returned to handlers.
- Files are grouped by domain concept (`certificate.go`, `organization.go`, `backup.go`), **not** by lifecycle category.

```go
type HelloWorld struct {
    ID        int64     `json:"id"`
    Message   string    `json:"message"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### `orm/`
GORM-mapped database rows.

- Files: `base.go`, `certificate.go`, `contact.go`, `example.go`, `organization.go`, `role.go`, `role_permission.go`, `user.go`.
- GORM tags only. Must implement `TableName()`.
- `base.go` holds shared embedded fields and primitives used across rows.
- Never exposed beyond the repository layer.
- Nullable FKs use `*int32` — avoid sentinel values like `ContactIDNotFound = 0`.

```go
type HelloWorld struct {
    ID        int64     `gorm:"primaryKey"`
    Message   string    `gorm:"type:text;not null"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
    UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (HelloWorld) TableName() string { return "helloworld_messages" }
```

### `repo/`
Per-domain repository interfaces, one file per domain.

- Files: `certificate.go`, `example.go`, `organization.go`, `role.go`, `user.go`.
- Implementations live in `pkg/services/repository/`.
- Methods take `context.Context` + entity option structs.
- `Create*`/`Get*`/`Update*`/`List*` return `*orm.*`; `Delete*` returns just `error`.
- Methods never accept `*orm.*` as input — services pass options, the repo builds the row.

```go
type User interface {
    CreateUser(ctx context.Context, opts entity.UserCreateOptions) (*orm.User, *orm.Contact, error)
    GetUser(ctx context.Context, userID int32) (*orm.User, *orm.Contact, error)
    ListUsers(ctx context.Context, opts entity.UserListOptions) ([]*orm.User, []*orm.Contact, error)
    UpdateUser(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*orm.User, *orm.Contact, error)
    DeleteUser(ctx context.Context, userID int32) error
    DeleteUsers(ctx context.Context, userIDs []int32) error
    ResetUserPassword(ctx context.Context, userID int32, plainPassword string) error
    GetUserByName(ctx context.Context, organizationID int32, username string) (*orm.User, error)
    GetContact(ctx context.Context, contactID int32) (*orm.Contact, error)
}
```

### `model/`
Per-domain business interfaces — the `Service` contracts consumed by routers and other services.

- Files: `auth.go`, `certificate.go`, `example.go`, `organization.go`, `role.go`, `user.go`.
- Methods take/return **`entity.*` only** — orm types never appear here.
- Method shapes mirror their implementation files; group with `// business.go` / `// lifecycle.go` comments.

```go
type User interface {
    common.Service
    // business.go
    Create(ctx context.Context, opts entity.UserCreateOptions) (*entity.User, error)
    List(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error)
    Get(ctx context.Context, userID int32) (*entity.User, error)
    Update(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error)
    Delete(ctx context.Context, userIDs []int32) error
    ResetPassword(ctx context.Context, isResetOwnPwd bool, userID int32, opts entity.UserResetPasswordOptions) error
    // lifecycle.go
    common.Initializable
}
```

### `infra/`
Infrastructure-level types shared across services — config schema, transport-agnostic plumbing.

- Files: `config.go`.
- Free of business semantics; safe to import from anywhere.

### `message/`
Pub/sub message payloads emitted by services and consumed by `MessageHandler` implementations.

- Files: `example.go`, `system.go`, `user.go`.
- One file per producing domain. Struct names describe the event (`DeleteLocalUsers`, not `DeleteUsersMessage`).
- Plain data — no behaviour, no orm/api tags.

### `preset/`
Default constants and seed values used during `Init()` bootstrap and tests.

- Files: `auth.go`, `certificate.go`, `organization.go`, `user.go`.
- Examples: `DefaultOrganizationID`, `AdminUsername`, `AdminPassword`, default CA settings.
- Referenced from service `Init()` so bootstrapping stays declarative.

### `rbac/`
RBAC primitives — features, permissions, roles — independent of storage.

- Files: `feature.go`, `permission.go`, `role.go`.
- Defines the enum-shaped vocabulary (`RoleAdmin`, permission actions/resources, feature flags) used by `model/auth` and the permission checks in middlewares.

## Naming Conventions

### Go acronyms — ALL CAPS

Acronyms in identifiers are always uppercase: `ID`, `URL`, `CAID`, `CertID`, `RestAPI`. Never `Id`, `Url`, `CAId`, `CertId`, `RestApi`.

### Parameter names — entity-qualified

Use entity-qualified parameter names (`roleID`, `userID`, `certID`, `roleName`, `organizationName`) in **all** method signatures across `model/`, `repo/`, and their impls. Reasons:

- Reads cleanly for both humans and AI tools.
- Disambiguates relationship-style methods like `GetPermissions(roleID)` (where pure `id` could be misread as permission ID).
- Cross-entity parameters become obvious: `GetUserByName(organizationID int32, username string)`.

Plural ID slices follow the same rule: `userIDs`, `certIDs`.

### Options structs — `*Options` (plural)

Always `Options`, never `Option`: `UserCreateOptions`, `CertCreateOptions`, `RoleUpdateOptions`. The struct name says what data it carries, not "an option".

### Verbs — business intent at service, CRUD at repo

| Layer | Verb style | Example |
|---|---|---|
| `model/` (service) | Business intent | `Import`, `Issue`, `Login`, `ResetPassword` |
| `repo/` (persistence) | CRUD primitives | `CreateCertificate`, `GetUser`, `UpdateRole` |

The service's `Import` may call the repo's `CreateCertificate` — same operation, different vocabularies. Don't push business verbs (`Import`/`Issue`) into the repo.

### `By*` qualifier rule

- Default lookup (by ID) takes no suffix: `GetRole(roleID)`.
- Name-keyed variants use the `By*` suffix: `GetRoleByName(name)`, `ListRolePermissionsByName(name)`.
- Never use explicit `ByID` — the suffix-less form *is* the ID variant.

### Pluralization matches the return shape

`GetPermissions(roleID) []string` (plural — returns a list). `CheckPermissionByName(name, action, resource) bool` (singular — checks one entry).

### JSON tags — snake_case

Use snake_case for all JSON tags: `"first_name"`, `"is_ca"`, `"start_time"`. Never camelCase mixed across entities.

## Layer Boundaries

### The orm boundary is the repo

- **Below the boundary** (repo implementations): orm types are fine. Build/transform them freely.
- **At the boundary** (repo interface): inputs are `entity.*Options`; outputs are `*orm.*` so services can extract specific fields.
- **Above the boundary** (services, handlers, middlewares): use `entity.*` exclusively. Never reach into orm types.

When a service needs an orm-only field (e.g. `user.Password` for bcrypt), it's acceptable to call `m.repository.GetUser(...)` directly — but this is the exception, not the rule.

### Service-side: prefer `m.Get` over `m.repository.Get*`

For internal service code that only needs entity-level fields:

```go
// Good — stays in entity layer
user, err := m.Get(ctx, userID)
msg.Username = user.Username

// Only when orm-only fields are needed (e.g. Password)
user, _, err := m.repository.GetUser(ctx, userID)
if bcrypt.CompareHashAndPassword([]byte(user.Password), ...) { ... }
```

## Repository Conventions

### `Delete*` returns just `error`

The repo's delete methods do not return the deleted record. When a service needs the deleted data (e.g. to emit a deletion event), it pre-fetches inside a transaction:

```go
func (m *manager) Delete(ctx context.Context, userIDs []int32) error {
    msg := message.DeleteLocalUsers{Usernames: []string{}}
    err := m.repository.Transaction(ctx, func(tctx context.Context) error {
        for _, userID := range userIDs {
            user, err := m.Get(tctx, userID)
            if err != nil { return errors.Wrap(err) }
            if err := m.repository.DeleteUser(tctx, userID); err != nil {
                return errors.Wrap(err)
            }
            msg.Usernames = append(msg.Usernames, user.Username)
        }
        return nil
    })
    if err != nil { return errors.Wrap(err) }
    m.sender.SendMessage(ctx, m, msg)
    return nil
}
```

This keeps the repo CRUD-shaped and the orm types inside its boundary.

### No bootstrap methods in the repo

Bootstrapping default data (admin user, default roles, default CA) is **application logic**, not persistence. It belongs in each service's `Init()`. The repo stays focused on CRUD primitives:

```go
func (m *manager) Init(ctx context.Context) error {
    if _, err := m.repository.GetUserByName(ctx, preset.DefaultOrganizationID, preset.AdminUsername); err == nil {
        return nil
    } else if !errors.Is(err, errors.NotFound) {
        return errors.Wrap(err)
    }
    if _, _, err := m.repository.CreateUser(ctx, entity.UserCreateOptions{
        OrganizationID:       preset.DefaultOrganizationID,
        Username:             preset.AdminUsername,
        Password:             preset.AdminPassword,
        Role:                 rbac.RoleAdmin,
        RequirePasswordReset: true,
    }); err != nil {
        return errors.Wrap(err)
    }
    return nil
}
```

Bootstrap goes in `Init()`, not `Start()`. Init runs synchronously before any Daemon starts, fails fast, and re-runs idempotently on restart.

### Use `Transaction(ctx, fn)` for atomic multi-call sequences

Services never touch raw DB. When you need atomicity across multiple repo calls, use the repo's transaction wrapper.

## Service Conventions

### Methods take/return `entity.*` only

Service interfaces in `model/` never reference `orm.*` types. The service is responsible for converting via a `toEntity` helper (`pkg/services/system/<svc>/util.go`).

### Lifecycle interfaces are composed

Implement only what you need: `common.Initializable`, `common.Daemon`, `common.Debuggable`, `common.MessageHandler`. Don't implement empty `Start()`/`Stop()` if the service isn't a daemon.

### File layout per service

```
pkg/services/system/<svc>/
├── manager.go    # struct, New(), Name(), Dependencies()
├── model.go      # Manager interface, labeled // business.go / // lifecycle.go
├── option.go     # functional options
├── business.go   # business methods + their private helpers
├── lifecycle.go  # Init / Start / Stop / Info / HandleMessage
└── util.go       # pure free-function converters (e.g. toEntity)
```

- **Private business helpers stay in `business.go`** — they participate in business workflows.
- **Pure converters go to `util.go`** as free functions (no `m *manager` receiver) when state isn't needed.
- **One-liner private wrappers** (e.g. `func (m *manager) get(id) (*orm.X, error) { return m.repository.GetX(id) }`) are forbidden — inline them at the call site.

## Error handling

All errors use `github.com/xhanio/errors`. Always wrap with `errors.Wrap(err)` — never return a raw error from a called function. Use category constructors at boundaries: `errors.BadRequest.Newf(...)`, `errors.NotFound.Wrapf(err, ...)`.

## Putting It Together

```go
// Service — takes/returns entity only
func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.HelloWorld, error) {
    ormModel, err := m.repository.CreateHelloWorld(ctx, message)
    if err != nil {
        return nil, errors.Wrap(err)
    }
    return toEntity(ormModel), nil
}

// Repo — builds the orm row internally, returns it for the service to use
func (m *manager) CreateHelloWorld(ctx context.Context, message string) (*orm.HelloWorld, error) {
    tx := m.db.FromContext(ctx)
    ormModel := &orm.HelloWorld{Message: message}
    if err := tx.Create(ormModel).Error; err != nil {
        return nil, errors.DBFailed.Wrap(err)
    }
    return ormModel, nil
}

// Converter — free function in util.go
func toEntity(m *orm.HelloWorld) *entity.HelloWorld {
    if m == nil { return nil }
    return &entity.HelloWorld{
        ID:        m.ID,
        Message:   m.Message,
        CreatedAt: m.CreatedAt,
        UpdatedAt: m.UpdatedAt,
    }
}

// Handler — parses api type, calls service, returns entity as JSON
func (r *router) Example(c echo.Context) error {
    var req api.CreateHelloWorldMessage
    if err := c.Bind(&req); err != nil {
        return errors.BadRequest.Newf("invalid request: %v", err)
    }
    if err := c.Validate(&req); err != nil {
        return errors.Wrap(err)
    }
    body, err := r.em.HelloWorld(c.Request().Context(), req.Message)
    if err != nil {
        return errors.Wrap(err)
    }
    return c.JSON(http.StatusOK, body)
}
```
