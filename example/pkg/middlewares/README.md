# Example Middlewares

This directory contains example HTTP middleware implementations demonstrating the Framingo middleware architecture and request processing patterns.

## Overview

Middlewares wrap HTTP handlers to perform cross-cutting concerns such as authentication, logging, compression, validation, and rate limiting. Each middleware in this directory is a self-contained package implementing the `api.Middleware` interface.

## Structure

```
middlewares/
├── authnagent/      # Authenticates agents via mTLS client certificate
├── authnuser/       # Authenticates users via API token or session
├── authz/           # Enforces RBAC permission checks on authenticated requests
├── deflate/         # Decompresses deflate-encoded request bodies
└── feature/         # Gates handlers behind feature licensing
```

Each package contains a `middleware.go` (and `option.go` when the middleware accepts a logger) implementing the `api.Middleware` interface.

## Packages

### [authnagent](authnagent/)

Authenticates agent requests by validating an X.509 client certificate against the default CA. Accepts certificates either directly via mTLS (`r.TLS.PeerCertificates`) or forwarded by nginx in the `X-Ssl-Certificate` header (`fapi.HeaderKeyClientCert`). Parses the agent ID from the certificate Common Name (`CN=<id>`) and writes a `*entity.Credential` (with `Source: AuthSourceAgent`) into the echo store under `fapi.ContextKeyCredential`. Handlers using `api.Context` read it via `c.Value(api.ContextKeyCredential)` or `c.Credential()`.

- Dependencies: [certificate.Manager](../services/system/certificate/)
- Options: `WithLogger(log.Logger)`

### [authnuser](authnuser/)

Authenticates user requests via one of two strategies:
- **API token** — if `fapi.HeaderKeyAPIToken` is set, calls `auth.AuthenticateAPIToken` and resolves role permissions via `role.GetPermissionsByName`
- **Session** — otherwise reads the session ID from `fapi.HeaderKeySession`, the `fapi.QueryParamSession` query parameter, or the `fapi.CookiesKeySession` cookie; looks up the session, refreshes its lease (`preset.SessionExpiration`), and resolves role permissions

Writes the resolved credential into the echo store under `fapi.ContextKeyCredential` (and the session under `fapi.ContextKeySession` when applicable). Handlers using `api.Context` consume it via `c.Value(api.ContextKeyCredential)` or `c.Credential()`.

- Dependencies: [auth.Manager](../services/system/auth/), [role.Manager](../services/system/role/)
- Options: `WithLogger(log.Logger)`

### [authz](authz/)

Enforces authorization on requests that already have a `*entity.Credential` in the context (placed there by `authnuser` or `authnagent`). Rejects credentials flagged with `RequirePasswordReset`, and for credentials from `AuthSourceLocalUser` or `AuthSourceLdapUser` verifies the role has permission for the current HTTP method and escaped path via `role.CheckPermissionByName`.

- Dependencies: [role.Manager](../services/system/role/)

### [deflate](deflate/)

Swaps the request body with a `zlib.Reader` when the request advertises `Content-Encoding: deflate`, so downstream handlers read decompressed bytes transparently.

- Dependencies: none

### [feature](feature/)

Gates handler execution behind a feature license. Reads the matched handler from `common.ContextKeyAPIRequestInfo` (populated by the API server), allows the request through when the handler has no `Permission`, and otherwise rejects it with `Forbidden` unless the permission appears in `rbac.Features[rbac.FeatureBasic]`.

- Dependencies: none

## Usage

### Creating a Middleware Instance

```go
import "github.com/xhanio/framingo/example/pkg/middlewares/deflate"

mw := deflate.New()
```

### Registering with the API Server

```go
import (
    "github.com/xhanio/framingo/pkg/services/api/server"
    "github.com/xhanio/framingo/example/pkg/middlewares/deflate"
)

srvMgr := server.New()

if err := srvMgr.RegisterMiddlewares(deflate.New()); err != nil {
    return errors.Wrap(err)
}
```

### Referencing in Router Config

The middleware name from `Name()` is the key used in [router.yaml](../routers/example/router.yaml):

```yaml
handlers:
  - method: POST
    path: /helloworld/:message
    func: Example
    middlewares: [deflate]   # name returned by middleware.Name()
```

## Middleware Implementation

The `deflate` middleware swaps the request body with a streaming decompressor when the client advertises deflate encoding:

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        r := c.Request()
        if r.Header.Get("Content-Encoding") == "deflate" {
            reader, err := zlib.NewReader(r.Body)
            if err != nil {
                return errors.BadRequest.Newf("failed to deflate request body: %s", err)
            }
            c.Request().Body = reader
        }
        return next(c)
    }
}
```

### How It Works

1. **Inspect headers** — checks `Content-Encoding` on the incoming request
2. **Decompress on match** — constructs a `zlib.Reader` over the original body
3. **Replace the body** — downstream handlers read decompressed bytes transparently
4. **Continue the chain** — calls `next(c)` to invoke the next middleware or the handler
5. **Error path** — returns `errors.BadRequest` if the body is not a valid deflate stream

## Middleware Interface

Defined in [pkg/types/api/model.go](../../../pkg/types/api/model.go):

```go
type Middleware interface {
    common.Service                            // Name() and Dependencies()
    Func(echo.HandlerFunc) echo.HandlerFunc   // standard Echo middleware signature
}
```

Every middleware must implement:
- `Name()` — unique identifier referenced from router YAML
- `Dependencies()` — services that must be initialized before this middleware
- `Func()` — the wrapping function applied to handlers

## Example Request Flow

A client sending a deflate-encoded POST request:

```bash
curl -X POST http://localhost:8080/example/helloworld/hi \
  -H "Content-Encoding: deflate" \
  --data-binary @payload.bin
```

Processing order:
1. Server middlewares (Recover, Info, Logger, Error, Throttle)
2. **deflate middleware** decompresses the body
3. The `Example` handler reads the decompressed body
4. The response flows back through the middleware chain

## Creating Custom Middlewares

### 1. Define the Struct and Factory

```go
package mymiddleware

import (
    "path"

    "github.com/labstack/echo/v4"

    "github.com/xhanio/framingo/pkg/types/api"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ api.Middleware = (*middleware)(nil)

type middleware struct{}

func New() api.Middleware {
    return &middleware{}
}
```

### 2. Implement Required Methods

```go
func (m *middleware) Name() string {
    pkg, _ := reflectutil.Locate(m)
    return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
    return nil
}
```

### 3. Implement Middleware Logic

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // pre-processing

        err := next(c)

        // post-processing

        return err
    }
}
```

### 4. Register and Reference

```go
// in components/server/.../api.go
srvMgr.RegisterMiddlewares(mymiddleware.New())
```

```yaml
# in routers/.../router.yaml
handlers:
  - method: GET
    path: /endpoint
    func: Handler
    middlewares:
      - mymiddleware
```

## Common Middleware Patterns

### Authentication

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        token := c.Request().Header.Get("Authorization")
        if !validateToken(token) {
            return errors.Unauthorized.Newf("invalid token")
        }
        c.Set("user", getUserFromToken(token))
        return next(c)
    }
}
```

### Request Validation

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        if c.Request().ContentLength > maxSize {
            return errors.BadRequest.Newf("request too large")
        }
        return next(c)
    }
}
```

### Response Modification

```go
func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        err := next(c)
        c.Response().Header().Set("X-Custom-Header", "value")
        return err
    }
}
```

## Best Practices

1. **Naming** — let `Name()` derive from the package name with `reflectutil.Locate` so it stays in sync with the directory
2. **Dependencies** — declare every service the middleware reads from, so the supervisor initializes them first
3. **Errors** — return categorized errors from `github.com/xhanio/errors` so the Error middleware maps them to the right HTTP status
4. **Performance** — keep middleware logic lightweight; heavy work belongs in services
5. **Order** — registration order on the server determines execution order for custom middlewares
6. **Context** — use `c.Set()` / `c.Get()` to pass data between middlewares and handlers
7. **Short-circuit** — return without calling `next(c)` to halt the chain (e.g., auth failures)

## See Also

- [API Server Middleware](../../../pkg/services/api/server/middleware.go)
- [API Server Manager](../../../pkg/services/api/server/manager.go)
- [API Types](../../../pkg/types/api/model.go)
- [Example Router](../routers/example/)
- [Echo Middleware Guide](https://echo.labstack.com/middleware/)
