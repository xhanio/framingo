# Example Command Component

This directory contains a CLI (Command Line Interface) implementation using Cobra, providing a command-line interface for the Framingo server application.

## Overview

The `example` cmd component provides a user-friendly CLI for managing the server application. It includes commands for running the server as a daemon, checking version information, and handling command-line arguments and flags.

## Structure

```
example/
├── root.go      # Root command and global flags
├── daemon.go    # Daemon command to run the server
└── common.go    # Common commands (version, etc.)
```

## Files

### [root.go](root.go)

Defines the root command and global persistent flags:
- Root command configuration
- Global flags: `--help`, `--verbose`
- Subcommand registration
- Pre-run hooks

### [daemon.go](daemon.go)

Implements the daemon command to start the server:
- Configuration file path flag (`--config`, `-c`)
- Server initialization and startup
- Integration with [pkg/components/server](../server/example/)

### [common.go](common.go)

Provides common utility commands:
- `version` command - displays build information
- JSON-formatted version output

## Usage

### Integration with main.go

Create a `main.go` file to use the CLI:

```go
package main

import (
    "fmt"
    "os"

    "github.com/xhanio/framingo/example/pkg/components/cmd/example"
)

func main() {
    rootCmd := example.NewRootCmd()
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Building the Binary

```bash
# Build the application
go build -o myapp cmd/myapp/main.go

# Or with build info
go build -ldflags="-X github.com/xhanio/framingo/pkg/types/info.Version=1.0.0 \
                    -X github.com/xhanio/framingo/pkg/types/info.Commit=$(git rev-parse HEAD) \
                    -X github.com/xhanio/framingo/pkg/types/info.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
         -o myapp cmd/myapp/main.go
```

## Commands

### daemon

Starts the server as a daemon process.

**Usage:**
```bash
myapp daemon [flags]
```

**Flags:**
- `-c, --config <path>` - Path to configuration file (default: `config.json`)
- `-v, --verbose` - Enable verbose output
- `--help` - Show help information

**Examples:**
```bash
# Start with default config
./myapp daemon

# Start with custom config
./myapp daemon -c /etc/myapp/config.yaml

# Start with verbose logging
./myapp daemon -c config.yaml -v
```

**What it does:**
1. Loads configuration from specified file
2. Creates server instance ([pkg/components/server](../server/example/))
3. Initializes all services, routers, and middlewares
4. Starts HTTP API server(s)
5. Blocks until SIGINT (Ctrl+C)
6. Gracefully shuts down

### version

Displays version and build information.

**Usage:**
```bash
myapp version
```

**Example Output:**
```json
{
  "product": "framingo",
  "version": "1.0.0",
  "commit": "a1b2c3d4e5f6",
  "build_time": "2024-01-15T10:30:00Z",
  "go_version": "go1.21.5"
}
```

**What it does:**
- Retrieves build information from [pkg/types/info](../../../../pkg/types/info/)
- Formats as JSON with indentation
- Prints to stdout

## Global Flags

These flags are available for all commands:

### --help
Shows help information for the command.

```bash
myapp --help
myapp daemon --help
```

### -v, --verbose
Enables verbose output (currently defined but not actively used).

```bash
myapp daemon -v
```

## Command Flow

### Daemon Command Flow

```
User runs: ./myapp daemon -c config.yaml
    
Root Command (PersistentPreRun)
  - Check --help flag
  - Set up global state
    
Daemon Command (RunE)
  - Parse --config flag
  - Create server with config path
    
Server Initialization (pkg/components/server)
  - Load YAML config via Viper
  - Initialize logger
  - Initialize database
  - Initialize services
  - Initialize API server(s)
  - Register middlewares (pkg/middlewares)
  - Register routers (pkg/routers)
    
Server Start
  - Start all services
  - Start HTTP API server(s)
  - Enable pprof (if configured)
  - Set up signal handlers
  - Block until SIGINT
    
Graceful Shutdown
  - Stop all services
  - Close connections
  - Exit
```

## Error Handling

The CLI implements proper error handling:

### Error Propagation
Errors from the server are wrapped and returned:
```go
if err := m.Init(); err != nil {
    return errors.Wrap(err)
}
```

### Silent Usage on Errors
When an error occurs, usage information is not shown:
```go
cmd := &cobra.Command{
    SilenceUsage: true,  // Don't show usage on runtime errors
}
```

### Exit Codes
- `0` - Success
- `1` - Error (automatically set by Cobra on error)

## Adding New Commands

### 1. Create Command File

Create a new file (e.g., `migrate.go`):

```go
package example

import (
    "github.com/spf13/cobra"
    "github.com/xhanio/errors"
)

func NewMigrateCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:          "migrate",
        Short:        "Run database migrations",
        RunE:         runMigrate,
        SilenceUsage: true,
    }
    return cmd
}

func runMigrate(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}
```

### 2. Register in Root Command

Add to [root.go](root.go):

```go
root.AddCommand(NewDaemonCmd())
root.AddCommand(NewVersionCmd())
root.AddCommand(NewMigrateCmd())  // Add new command
```

### 3. Use the Command

```bash
myapp migrate
```

## Common Command Patterns

### Command with Arguments

```go
func NewExampleCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "example [name]",
        Short: "Example command with arg",
        Args:  cobra.ExactArgs(1),  // Require exactly 1 arg
        RunE: func(cmd *cobra.Command, args []string) error {
            name := args[0]
            fmt.Printf("Hello, %s!\n", name)
            return nil
        },
    }
    return cmd
}
```

### Command with Multiple Flags

```go
var (
    host string
    port int
    tls  bool
)

func NewServerCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "server",
        RunE: runServer,
    }
    cmd.Flags().StringVar(&host, "host", "0.0.0.0", "Server host")
    cmd.Flags().IntVar(&port, "port", 8080, "Server port")
    cmd.Flags().BoolVar(&tls, "tls", false, "Enable TLS")
    return cmd
}
```

### Subcommands

```go
func NewDatabaseCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "database",
        Short: "Database operations",
    }
    // Add subcommands
    cmd.AddCommand(NewMigrateCmd())
    cmd.AddCommand(NewSeedCmd())
    return cmd
}

// Usage: myapp database migrate
// Usage: myapp database seed
```

## Configuration Priority

When using the daemon command, configuration is loaded with priority:

1. **Command-line flags** (highest priority)
2. **Environment variables** (with product prefix)
3. **Configuration file** (specified via `-c` flag)
4. **Default values** (lowest priority)

Example:
```bash
# Config file specifies port: 8080
# Environment variable overrides it
export FRAMINGO_API_HTTP_PORT=9090

# Command runs with port 9090
./myapp daemon -c config.yaml
```

## Best Practices

1. **Consistent Naming**: Use verb-noun pattern (e.g., `list users`, `create user`)
2. **Help Text**: Provide clear, concise help messages
3. **Error Messages**: Return descriptive errors with context
4. **Flags**: Use short flags (`-c`) for common options
5. **Validation**: Validate flags before executing command logic
6. **Silent Usage**: Set `SilenceUsage: true` for runtime errors
7. **Exit Codes**: Use appropriate exit codes (0 for success, non-zero for errors)

## Integration with Server Component

The daemon command integrates directly with the server component:

```go
func runDaemon(cmd *cobra.Command, args []string) error {
    // Create server from pkg/components/server
    m := example.New(example.Config{
        Path: configPath,
    })

    // Initialize (loads config, sets up services)
    if err := m.Init(); err != nil {
        return errors.Wrap(err)
    }

    // Start (blocks until SIGINT)
    if err := m.Start(context.Background()); err != nil {
        return errors.Wrap(err)
    }

    return nil
}
```

This connects:
- CLI layer (this package)
- Server component ([pkg/components/server](../server/example/))
- All registered services, routers, and middlewares

## Development Workflow

### Running During Development

```bash
# Run directly with go run
go run cmd/myapp/main.go daemon -c config.yaml

# Run with verbose flag
go run cmd/myapp/main.go daemon -c config.yaml -v

# Check version
go run cmd/myapp/main.go version
```

### Testing Commands

```bash
# Build and test
go build -o myapp cmd/myapp/main.go
./myapp --help
./myapp daemon --help
./myapp version
```

### Debugging

```bash
# Use delve for debugging
dlv debug cmd/myapp/main.go -- daemon -c config.yaml
```

## Production Deployment

### Systemd Service

Create a systemd service file (`/etc/systemd/system/myapp.service`):

```ini
[Unit]
Description=My Framingo Application
After=network.target

[Service]
Type=simple
User=myapp
Group=myapp
WorkingDirectory=/opt/myapp
ExecStart=/opt/myapp/bin/myapp daemon -c /etc/myapp/config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable myapp
sudo systemctl start myapp
sudo systemctl status myapp
```

### Docker Container

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o myapp cmd/myapp/main.go

FROM debian:bookworm-slim
COPY --from=builder /app/myapp /usr/local/bin/
COPY config.yaml /etc/myapp/config.yaml
CMD ["myapp", "daemon", "-c", "/etc/myapp/config.yaml"]
```

## See Also

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Server Component](../server/example/)
- [Example Services](../../services/example/)
- [Example Routers](../../routers/example/)
- [Example Middlewares](../../middlewares/example/)
- [Build Info Types](../../../../pkg/types/info/)
