# Clients — HTTP Client and the Client Component

Two layers, mirroring the server side: framingo's `pkg/services/api/client`
is the transport (TLS, headers, cookies, encoding, structured errors), and
the project's `pkg/components/client/<app>/` wraps it into an app-specific
SDK the CLI and other programs consume.

## Framework HTTP Client — `pkg/services/api/client`

```go
import "github.com/xhanio/framingo/pkg/services/api/client"

cli := client.New("https://api.example.com",
    client.WithLogger(logger),
    client.WithTimeout(10*time.Second),
    // client.WithCert(certBundle, tls.RequireAndVerifyClientCert), // mTLS
    // client.WithDebug(),                                          // skip TLS verify - dev only
)
```

```go
type Client interface {
    common.Initializable
    SetHeaders(headers ...common.Pair[string, string])   // global; empty value deletes
    SetCookies(cookies ...*http.Cookie)
    NewRequest(ctx context.Context, request *Request, opts ...RequestOption) (*http.Request, error)
    Do(req *http.Request) (*http.Response, error)
    Send(ctx context.Context, request *Request, opts ...RequestOption) (*http.Response, error)  // NewRequest + Do
}
```

A `client.Request` carries `Method`, `Path`, `Headers` (`common.Pairs`),
`Cookies`, `ContentType`, `Body` (an `io.Reader`, `[]byte`, `string`, or —
with a JSON content type — any marshalable value), and `Encoding`
(`api.EncodingDeflate` compresses the body; the server's deflate middleware
inflates it). Per-request options: `WithRequestHeaders`, `WithRequestCookies`,
`WithRequestEncoding`.

Global headers and cookies set via `SetHeaders`/`SetCookies` ride every
request — that's how a session token attaches once after login.

## Project Client Component — `pkg/components/client/<app>/`

The example ships one at `example/pkg/components/client/example/`: an
app-facing SDK that owns the endpoint, credentials, and one method per API
operation, layered over `client.Client`.

```
pkg/components/client/example/
  model.go      # the component's exported interface
  client.go     # New(opts...) + the wrapped client.Client + session header wiring
  option.go     # WithEndpoint / WithDebug / ... functional options
  auth.go       # Login/Logout/Session operations; stores the credential,
                #   then SetHeaders(common.NewPair(fapi.HeaderKeySession, sessionID))
  example.go    # one file per API domain, one method per operation
```

The pattern to copy: operations take `ctx` and typed DTOs (from the project's
`pkg/types/api`), build a `client.Request`, `Send`, and decode — callers never
see `*http.Request`. Auth state lives in the component; after login it
installs the session header globally so every subsequent call carries it.

The CLI under `pkg/components/cmd/` consumes this component, not
`client.Client` directly — see [package-layout.md](package-layout.md) for
where components sit in the tree.
