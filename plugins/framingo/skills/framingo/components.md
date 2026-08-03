# Components — cmd, server, client

`pkg/components/` is the application-wiring category: the code that assembles
services into a runnable program. Three kinds, each in its own subtree:

| Subtree | Role | Deep reference |
|---|---|---|
| `components/cmd/` | Cobra CLI wiring — one package per binary persona | this file |
| `components/server/<app>/` | The application daemon: config, service graph, API registration, signals | [package-layout.md](package-layout.md) "Server Component" |
| `components/client/<app>/` | The app-facing SDK over the HTTP client | [client.md](client.md) |

Binaries under `build/binary/<name>/main.go` are thin: construct the root
command, `Execute()`, exit non-zero on error. Everything real lives in the
component packages.

## `components/cmd/` — CLI Wiring

One package per binary persona. The example ships two: `cmd/app` (the daemon
binary) and `cmd/cli` (the operator CLI consuming the client component).

The daemon side is two small files — a root command and a `daemon`
subcommand that constructs the server component and runs it:

```go
// components/cmd/app/daemon.go
func NewDaemonCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "daemon", RunE: runDaemon, SilenceUsage: true}
    cmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.json", "config file path")
    return cmd
}

func runDaemon(cmd *cobra.Command, args []string) error {
    m := example.New(configPath)      // the server component
    ctx := context.Background()
    if err := m.Init(ctx); err != nil {
        return errors.Wrap(err)
    }
    if err := m.Start(ctx); err != nil {  // blocks until shutdown
        return errors.Wrap(err)
    }
    return nil
}
```

The CLI side builds the client component once in `PersistentPreRunE` — flags
resolve the endpoint, verbosity, and the credential file under the user's
home — then each subcommand file (one per API domain: `auth.go`,
`example.go`, ...) calls its methods:

```go
// components/cmd/cli/root.go (shape)
root := &cobra.Command{
    SilenceUsage: true,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        opts := []example.Option{
            example.WithCredential(credFile),   // ~/.example
            example.WithEndpoint(endpoint),
        }
        if verbose {
            opts = append(opts, example.WithDebug())
        }
        cli = example.New(opts...)
        return errors.Wrap(cli.Init())
    },
}
```

## `components/server/<app>/` — The Application Daemon

The heart of the wiring. The file-per-responsibility structure, layering
rules, and registration order are specified in
[package-layout.md](package-layout.md) — `model.go` / `manager.go` /
`lifecycle.go` / `config.go` / `service.go` / `api.go` / `signal.go`, with
the reference implementation at `example/pkg/components/server/example/`.
The short version:

- `service.go` builds every service in layered order (logger → infra → system
  → business → API manager with its per-server options from config).
- `api.go` constructs middlewares and routers and registers both with the API
  manager — middlewares first.
- `lifecycle.go` orchestrates: config → services → supervisor registration →
  `TopoSort` → `Init` → `initAPI` → `Start`, blocking on the context.
- `signal.go` is where SIGINT/SIGTERM/SIGHUP/SIGUSR1/SIGUSR2 handling lives —
  the supervisor deliberately installs no signal handlers
  ([services.md](services.md)).

## `components/client/<app>/` — The SDK

Covered in [client.md](client.md): a typed, credential-holding wrapper over
`pkg/services/api/client` with one method per API operation, consumed by
`cmd/cli` and any other program that talks to the app.
