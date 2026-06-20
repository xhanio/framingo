# Example Command Component

This directory contains the Cobra-based command-line entry points for the example project. It is split into two sibling packages that map to two binaries:

- [`app`](app/) drives the `exampleapp` daemon binary (long-running server process).
- [`cli`](cli/) drives the `examplecli` client binary (operator/user-facing subcommands that talk to the daemon via the client SDK).

## Structure

```
cmd/
├── app/             # exampleapp (daemon)
│   ├── root.go      # Root command and global flags
│   ├── daemon.go    # `daemon` subcommand: boots the server
│   └── common.go    # `version` and other shared subcommands
└── cli/             # examplecli (client)
    ├── root.go      # Root command, global flags, client SDK wiring
    ├── common.go    # `version` and other shared subcommands
    ├── auth.go      # `login` / `logout`
    ├── cert.go      # `certutil` (CA + server cert generation)
    └── example.go   # `helloworld` and other domain subcommands
```

## `app` package — `exampleapp` daemon

`app.NewRootCmd()` returns the root command for the daemon binary. It registers:

- `daemon` — loads YAML config (instance-based Viper), constructs the server from [pkg/components/server/example](../server/example/), runs `Init` then `Start`, and blocks until SIGINT/SIGTERM. Accepts `-c, --config <path>` (default `config.json`).
- `version` — prints build info from [pkg/types/info](../../../../pkg/types/info/) as JSON.

Global persistent flags: `--help`, `-v, --verbose`.

Wired into [`example/build/binary/exampleapp/main.go`](../../../build/binary/exampleapp/main.go):

```go
func main() {
    rootCmd := app.NewRootCmd()
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Daemon flow

```
exampleapp daemon -c config.yaml
  └── server.New(configPath)
        ├── Init(ctx)   # load config, init logger/db, build supervisor, register routers/middlewares
        └── Start(ctx)  # start services + HTTP API, install signal handlers, block until SIGINT/SIGTERM,
                        # then graceful shutdown in reverse dependency order
```

## `cli` package — `examplecli` client

`cli.NewRootCmd()` returns the root command for the client binary. Unlike `app`, this root command owns a shared client SDK instance ([pkg/components/client/example](../client/example/)) that all subcommands use.

The `PersistentPreRunE` hook runs before every subcommand and:

1. Resolves the credential file at `~/.example`.
2. Builds an `example.Client` with `WithCredential`, `WithEndpoint`, and (when `-v`) `WithDebug`.
3. Calls `cli.Init()` so subcommands can immediately invoke client methods.

Global persistent flags: `--help`, `-v, --verbose`, `-e, --endpoint <url>`.

### Subcommand groupings

- **Auth** ([auth.go](cli/auth.go))
  - `login -u <username>` — prompts for password via `term.ReadPassword`, calls `cli.Login`.
  - `logout` — calls `cli.Logout`.
- **Cert** ([cert.go](cli/cert.go))
  - `certutil` — generates a CA (`ca.crt`/`ca.key`) and a server cert (`server.crt`/`server.key`) via [pkg/utils/certutil](../../../../pkg/utils/certutil/). Flags: `-p, --product-cn`, `--domain`, `--ip`.
- **Domain** ([example.go](cli/example.go))
  - `helloworld [message]` — calls `cli.HelloWorld` against the server.
- **Common** ([common.go](cli/common.go))
  - `version` — same build-info output as the daemon's `version`.

Wired into [`example/build/binary/examplecli/main.go`](../../../build/binary/examplecli/main.go) with the same pattern as `exampleapp`, swapping `app.NewRootCmd()` for `cli.NewRootCmd()`.

## Adding new subcommands

Pick the package by audience:

- Operates on the running process (or is intrinsic to the server) → add to `app`.
- Talks to the server as a client / produces local artifacts → add to `cli` and use the package-level `cli` client instance set up by `PersistentPreRunE`.

Then register the new `*cobra.Command` in the corresponding `root.go` via `root.AddCommand(...)`.

## Configuration priority (daemon)

Standard Viper precedence:

1. Command-line flags
2. Environment variables (product prefix)
3. Configuration file (via `-c`)
4. Defaults

## See Also

- [Server Component](../server/example/) — what `app daemon` boots
- [Client Component](../client/example/) — what `cli` subcommands call
- [Example Routers](../../routers/example/)
- [Example Services](../../services/example/)
- [Build Info Types](../../../../pkg/types/info/)
- [`exampleapp` main](../../../build/binary/exampleapp/main.go)
- [`examplecli` main](../../../build/binary/examplecli/main.go)
