# Routers — Authoring HTTP Handlers

The project-side router pattern: the `router.go`/`handler.go`/`router.yaml`
triple, the project `api.Context`, and handler discovery. For the server that
consumes routers — registration flow, YAML schema, middleware resolution,
WebSockets — see [api-server.md](api-server.md).

Routes are defined declaratively: each `fapi.Router` ships an embedded `router.yaml` plus a `Handlers()` map; the server manager binds them at registration time. The recommended layout splits each package into `router.go` (factory + `Handlers()` boilerplate) and `handler.go` (handler bodies). Templates: [`_templates/router.go`](_templates/router.go), [`_templates/handler.go`](_templates/handler.go), [`_templates/router.yaml`](_templates/router.yaml).

`New` returns `fapi.Router`; the unexported `newRouter` returns the concrete `*router`, so package tests construct it directly — the convention every router and service follows.

## Two `api` packages — don't confuse them

Router code imports both. The example project's convention (follow it):

```go
import (
    fapi "github.com/xhanio/framingo/pkg/types/api"  // FRAMEWORK: Router, Middleware, ContextKey*, ErrorBody
    "myapp/pkg/types/api"                            // PROJECT (yours): Context, DiscoverHandlers, DTOs
)
```

- `fapi` — **framingo's** `pkg/types/api`. Has `Router`, `Middleware`, `RequestInfo`, `CORSConfig`, `ErrorBody`, `ContextKeyCredential`. It has **no `Context` type**.
- `api` — **your project's** `pkg/types/api`, which you own and can extend. Defines `Context`, `DiscoverHandlers`, `WrapHandler`, `WrapWebSocket`, and request/response DTOs.

`api.Context` below always means the **project** one. Referring to it as a framingo type is a mistake — framingo ships no such interface.

## Handler signature — use the project `api.Context`

**When defining an API, write handlers as `func(c api.Context) error` — not `func(c echo.Context) error`.** `api.Context` is the interface *your project* defines (canonical version: [`_templates/api-context.go`](_templates/api-context.go)) that embeds `echo.Context` **and** `context.Context`, and adds project helpers:

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

If a project has no `pkg/types/api/api.go` yet (i.e. it wasn't forked from `example/`), copy [`_templates/api-context.go`](_templates/api-context.go) into it before writing handlers, and adjust the `entity` import to the project's own.

## `Handlers()` — call `DiscoverHandlers` in each `router.go`

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

Don't hand-write the map (`map[string]any{"ListUsers": r.ListUsers}`) — it forces `echo.HandlerFunc` signatures, defeating `api.Context`, and rots on rename. Keep the debug line: methods that don't match a known signature are skipped **silently**, so the count is your only startup signal. The example's routers pin the yaml↔handler mapping in a package test built on `newRouter` — every `func:` in the embedded `router.yaml` must resolve to a discovered handler.

## Wiring routers into the server

```go
import "github.com/xhanio/framingo/pkg/services/api/server"

srvMgr := server.New(server.WithLogger(logger))

// Add / RegisterMiddlewares / RegisterRouters ALL return error — check them.
// port is uint: use config.GetUint(...), not GetInt.
if err := srvMgr.Add("http", server.WithEndpoint("0.0.0.0", 8080, "/")); err != nil {
    return errors.Wrap(err)
}
if err := srvMgr.RegisterMiddlewares(authMW, throttleMW); err != nil { // before routers
    return errors.Wrap(err)
}
if err := srvMgr.RegisterRouters(userRouter, orderRouter); err != nil {
    return errors.Wrap(err)
}
```

For the full registration flow, router/middleware contracts, YAML format, handler key format, WebSocket handling, and middleware resolution, see [api-server.md](api-server.md).
