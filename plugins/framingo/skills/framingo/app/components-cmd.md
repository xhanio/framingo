# Cmd Component — Cobra CLI Wiring

`pkg/components/cmd/` holds one package per binary persona. The example ships
two — `cmd/app` (the daemon binary) and `cmd/cli` (the operator CLI) — and
the binaries under `build/binary/<name>/main.go` are deliberately thin:

```go
// build/binary/exampleapp/main.go — the whole file
func main() {
    rootCmd := app.NewRootCmd()
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

Everything real lives in the component packages; the `main` never grows.

## `cmd/app` — The Daemon Binary

Two small files: a root command that mounts subcommands, and a `daemon`
subcommand that constructs the [server component](components-server.md) and
runs it:

```go
// components/cmd/app/daemon.go
var configPath string

func NewDaemonCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:          "daemon",
        RunE:         runDaemon,
        SilenceUsage: true,
    }
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

`root.go` builds the root `cobra.Command` (with `SilenceUsage: true`) and
`AddCommand(NewDaemonCmd())` — plus whatever operational subcommands the app
grows (migrations, one-shot admin tasks).

## `cmd/cli` — The Operator CLI

The CLI consumes the [client component](components-client.md), never `client.Client`
directly. The pattern: build the SDK once in the root command's
`PersistentPreRunE` — flags resolve the endpoint, verbosity, and a credential
file under the user's home — then one file per API domain (`auth.go`,
`example.go`, ...) adds subcommands that call its methods:

```go
// components/cmd/cli/root.go (shape)
var (
    verbose  bool
    endpoint string
    credFile string

    cli example.Client // the client component, shared by all subcommands
)

func NewRootCmd() *cobra.Command {
    root := &cobra.Command{
        SilenceUsage: true,
        PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
            cu, err := user.Current()
            if err != nil {
                return errors.Wrap(err)
            }
            credFile = filepath.Join(cu.HomeDir, ".example") // session survives between runs
            opts := []example.Option{
                example.WithCredential(credFile),
                example.WithEndpoint(endpoint),
            }
            if verbose {
                opts = append(opts, example.WithDebug())
            }
            cli = example.New(opts...)
            return errors.Wrap(cli.Init())
        },
    }
    root.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "http://localhost:8080", "server endpoint")
    root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "debug output")
    // root.AddCommand(NewLoginCmd(), NewHelloworldCmd(), ...)
    return root
}
```

Subcommand files stay declarative: parse flags/args, call one SDK method,
print the result. Auth state (the credential file, the session header) is the
client component's job, not the CLI's.
