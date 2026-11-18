# HTTP Client — `pkg/services/api/client`

The framework transport: TLS, headers, cookies, body encoding, structured
errors. The app-facing SDK built on top of it lives in
[components-client.md](../app/components-client.md).

```go
import "github.com/xhanio/framingo/pkg/services/api/client"

cli := client.New("https://api.example.com/api/v1", // scheme, host and base path
    client.WithLogger(logger),
    client.WithTimeout(10*time.Second),
    client.WithCert(certBundle, tls.RequireAndVerifyClientCert), // TLS: required for https
    // client.WithDebug(), // debug-log each request's URL and headers, and the status
)
if err := cli.Init(ctx); err != nil { // builds the http.Client; required before any request
    return errors.Wrap(err)
}
```

`Init` builds the transport from the endpoint's scheme: plain `http.Client`
for `http://`, a TLS transport from `WithCert` for `https://` — an https
endpoint without `WithCert` is not supported. `WithCert`'s
`tls.ClientAuthType` doubles as the verification switch: `tls.NoClientCert`
skips verifying the server's certificate, anything else verifies it against
the bundle's CAs.

```go
type Client interface {
    common.Initializable
    SetHeaders(headers ...common.Pair[string, string])   // global
    SetCookies(cookies ...*http.Cookie)
    NewRequest(ctx context.Context, request *Request, opts ...RequestOption) (*http.Request, error)
    Do(req *http.Request) (*http.Response, error)
    Send(ctx context.Context, request *Request, opts ...RequestOption) (*http.Response, error)  // NewRequest + Do
}
```

A `client.Request` carries `Method`, `Path`, `Headers` (`common.Pairs`),
`Cookies`, `ContentType`, `Body` (an `io.Reader`, `[]byte`, `string`, or —
with a JSON content type — any marshalable value), and `Encoding`
(`api.EncodingDeflate` compresses the body; the example's `deflate` middleware
inflates it). Any other `Body` with a non-JSON content type sends no body.
Per-request options: `WithRequestHeaders`, `WithRequestCookies`,
`WithRequestEncoding`. `Path` is joined onto the endpoint with exactly one
`/` between them.

`Do` returns a 4xx/5xx as an error — a `*api.ErrorBody`, decoded from a
framingo server's body, else one with `Source: api.ErrorSourceUnknown` and
the raw body as `Message` — alongside the response, whose body is then
already drained. A transport error returns a nil response. How to turn the
`ErrorBody` back into an `xhanio/errors` category is in
[api.md](api.md#consuming-the-server-with-pkgservicesapiclient).

Global headers and cookies set via `SetHeaders`/`SetCookies` ride every
request — that's how a session token attaches once after login. A
per-request header of the same name overrides the global one; an empty
value (header) or an empty `Value` (cookie) deletes the global entry.
