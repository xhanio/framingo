# Middlewares — Authoring `api.Middleware`

How to write a middleware and attach it. The server's side of the story —
the chain order, config resolution, the built-ins — is in
[api-server.md](api-server.md).

## The Contract

One interface, one method (`pkg/types/api`, alias `fapi`):

```go
type Middleware interface {
    common.Service
    // config is the raw YAML written under the middleware's name, nil when
    // there is none. Called once per attachment at registration - and again
    // on restart, when routes are rebuilt - so per-route state lives in the
    // returned closure. An error fails registration; returning no function
    // and no error declines the attachment, and the server skips it.
    Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error)
}
```

Middlewares live in the project's `pkg/middlewares/<name>/`, one package per
middleware, unexported struct + `New()` factory. The name comes from the
package path — that's what router.yaml refers to:

```go
func (m *middleware) Name() string {
    pkg, _ := reflectutil.Locate(m)
    return path.Base(pkg) // package name == middleware name
}
```

## Config-Free Middleware

Most middlewares take no config. Refuse a block rather than ignore it — a
typo'd router.yaml block should fail startup, not silently do nothing:

```go
// Func implements api.Middleware. The middleware takes no router.yaml config,
// so a block under its name is a mistake worth failing startup for.
func (m *middleware) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
    if config != nil {
        return nil, errors.Newf("%s takes no config", m.Name())
    }
    return m.handle, nil
}

func (m *middleware) handle(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // ... work, then
        return next(c)
    }
}
```

Reference implementations in `example/pkg/middlewares/`: `deflate`
(request-body inflation with a decompression-bomb cap), `authnuser` (session
auth, sets `fapi.ContextKeyCredential`/`ContextKeySession`), `authz`
(permission check), `feature` (license gating).

## Configured Middleware

A middleware that takes config unmarshals its own block — the framework
carries it as raw bytes and never interprets it. Keep the block's shape as an
exported type in the project's `pkg/types/api`. Per-attachment state lives in
the closure `Func` returns; the example's `throttle` is the canonical case —
each attachment (each route) gets its own limiter table:

```go
func (m *middleware) Func(raw []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
    var cfg api.ThrottleConfig
    if raw != nil {
        if err := yaml.Unmarshal(raw, &cfg); err != nil {
            return nil, errors.Wrapf(err, "invalid throttle config")
        }
    }
    if cfg.RPS == 0 || cfg.BurstSize == 0 {
        // No limit for this route: decline-adjacent - pass everything.
        return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
    }
    var mu sync.RWMutex
    limits := make(map[string]*rate.Limiter) // per-route table, keyed by client IP
    return func(next echo.HandlerFunc) echo.HandlerFunc { /* ... */ }, nil
}
```

## Reading the Request

Route-attached middlewares run after the built-in Info middleware, so
`fapi.RequestInfo` is on the context — client IP, path, trace ID, and the
route's declared metadata flattened onto it (`Permission`, `Poll`):

```go
req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo)
if !ok || req == nil {
    return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
}
required := req.Permission // declared as `permission:` in router.yaml
```

Server-level middlewares (`WithMiddlewares`) run before routing —
`RequestInfo` does not exist there; a middleware that reads it belongs in
router.yaml.

## Attaching

- **Route-scoped** (the normal case): register with
  `srvMgr.RegisterMiddlewares(mw)` *before* routers, then reference by name
  in router.yaml at group or handler level — bare, or with a config block.
  Config resolves handler > group > server default > nil; see
  [api-server.md](api-server.md) for the entry forms and the server
  middleware configs mapping.
- **Server-level** (must see every request, before routing): pass to
  `server.Add(name, server.WithMiddlewares(mw))`. CORS is the built-in
  occupant of that position; `cors` is the one name the server claims.

Registration order matters: a router.yaml name not yet registered fails
`RegisterRouters` with `middleware <name> not found`.
