# Client Component — The App SDK

`pkg/components/client/<app>/` is the app-facing SDK: a typed,
credential-holding wrapper over the framework's
[HTTP client](../pkgs/client.md), with one method per API operation. The
[operator CLI](components-cmd.md) and any other program that talks to the app
consume this component, never `client.Client` directly.

The example ships one at `example/pkg/components/client/example/`:

```
pkg/components/client/example/
  model.go      # the component's exported interface
  client.go     # New(opts...) + the wrapped client.Client + session header wiring
  option.go     # WithEndpoint / WithCredential / WithDebug / ... functional options
  auth.go       # Login/Logout/Session operations; stores the credential,
                #   then SetHeaders(common.NewPair(fapi.HeaderKeySession, sessionID))
  example.go    # one file per API domain, one method per operation
```

The pattern to copy: operations take `ctx` and typed DTOs (from the project's
`pkg/types/api`), build a `client.Request`, `Send`, and decode — callers never
see `*http.Request`. Auth state lives in the component; after login it
installs the session header globally so every subsequent call carries it, and
the credential file (`WithCredential`) lets a session survive between CLI
runs.

See [layout.md](layout.md) for where components sit in the tree.
