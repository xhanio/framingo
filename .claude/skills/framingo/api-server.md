# Framingo API Server Reference

Full reference for the Framingo API server (`pkg/services/api/server`): the declarative routing model, registration flow, router and middleware contracts, YAML config format, handler keys, route mapping, WebSocket handlers, and middleware resolution.

## Route Registration Flow

```
1. Server instances created    →  srvMgr.Add("http", ...)
2. Middlewares registered      →  srvMgr.RegisterMiddlewares(authMW, ...)
3. Routers registered         →  srvMgr.RegisterRouters(myRouter)
   a. Router.Config() called  →  returns embedded YAML ([]byte)
   b. YAML unmarshaled        →  produces HandlerGroup with Handlers
   c. Router.Handlers() called →  returns map[string]any (echo.HandlerFunc or api.WebSocketHandlerFunc)
   d. For each YAML handler:
      - Func name looked up in Handlers() map
      - Type asserted based on method (WS → WebSocketHandlerFunc, others → echo.HandlerFunc)
      - HandlerKey struct generated: {Server, Method, Path}
      - Handler func stored in manager.handlerFuncs[key] or wsHandlerFuncs[key]
   e. Server matched by HandlerGroup.Server field
   f. Echo group created at endpoint.Path + group.Prefix
   g. For each handler:
      - Handler-specific + group-level middlewares collected
      - Route registered: group.Add(method, path, handlerFunc, middlewares...)
      - Handler metadata stored on server for runtime lookup
```

## Creating a Server Manager

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

## Implementing a Router

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
// Values must be echo.HandlerFunc for HTTP or api.WebSocketHandlerFunc for WS
func (r *router) Handlers() map[string]any {
    return map[string]any{
        "ListUsers":  echo.HandlerFunc(r.listUsers),
        "CreateUser": echo.HandlerFunc(r.createUser),
        "GetUser":    echo.HandlerFunc(r.getUser),
    }
}

func (r *router) listUsers(c echo.Context) error  { /* ... */ }
func (r *router) createUser(c echo.Context) error { /* ... */ }
func (r *router) getUser(c echo.Context) error    { /* ... */ }
```

## Router YAML Config Format (`router.yaml`)

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
  - method: WS            # WebSocket handler — registered as GET, server upgrades automatically
    path: /feed
    func: Feed            # must be api.WebSocketHandlerFunc in Handlers() map
```

## Handler Key Format

The server uses a struct-based key to uniquely identify each handler:

```go
type HandlerKey struct {
    Server string
    Method string
    Path   string
}
```

For example, the key for `ListUsers` is `HandlerKey{Server: "http", Method: "GET", Path: "/users"}`. Keys are used as map keys for direct lookups. The `matchHandler` logic falls back through: exact match → WS fallback (for GET requests) → ANY fallback → wildcard path matching.

## How Routes Map to Echo

The final Echo route path is: `server.endpoint.Path` + `group.Prefix` + `handler.Path`.

For a server added as `srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/"))` with the YAML above:
- `GET /users` → `ListUsers`
- `POST /users` → `CreateUser`
- `GET /users/:id` → `GetUser`

Root paths (`/`) are normalized by trimming the trailing slash, so both `/api/v1` and `/api/v1/` resolve to the same handler (via `RemoveTrailingSlash` pre-middleware).

## Router Interface (types)

```go
// pkg/types/api/model.go
type WebSocketHandlerFunc func(ctx context.Context, conn *websocket.Conn) error

type Router interface {
    common.Service
    Config() []byte                    // YAML config bytes (typically //go:embed)
    Handlers() map[string]any          // func name → echo.HandlerFunc or WebSocketHandlerFunc
}

type Middleware interface {
    common.Service
    Func(echo.HandlerFunc) echo.HandlerFunc   // standard Echo middleware signature
}
```

## WebSocket Handlers

Declare WebSocket routes with `method: WS` in YAML. The server automatically upgrades the connection and passes it to the handler. Uses `github.com/coder/websocket`.

```go
func (r *router) Handlers() map[string]any {
    return map[string]any{
        "ListUsers": echo.HandlerFunc(r.listUsers),
        "Feed":      api.WebSocketHandlerFunc(r.feed),
    }
}

func (r *router) feed(ctx context.Context, conn *websocket.Conn) error {
    for {
        typ, msg, err := conn.Read(ctx)
        if err != nil {
            return nil // client disconnected
        }
        if err := conn.Write(ctx, typ, msg); err != nil {
            return err
        }
    }
}
```

**Lifecycle**: Middleware stack runs before upgrade (auth, logging, throttle all apply). Once upgraded, errors are logged and the connection is closed with the appropriate status code. The handler receives a `context.Context` from the original HTTP request.

## Middleware Resolution

Middlewares are registered by name via `RegisterMiddlewares()`. The YAML config references them by the name returned from `Middleware.Name()`. During route registration:

1. Handler-specific middlewares are resolved first (from `handler.middlewares`)
2. Group-level middlewares are resolved next (from `group.middlewares`)
3. If any referenced middleware name is not found, registration fails with `NotImplemented` error

Built-in server middlewares (applied to all routes automatically):
`Request → CORS (debug only) → Recover → Logger → Info → Error → Throttle → [Custom middlewares] → Handler → Response`
