# Quick Start Guide

This guide shows how to quickly build and run the Framingo example application using GoPro.

## Prerequisites

1. Install GoPro:
   ```bash
   go install github.com/xhanio/gopro@latest
   ```

2. Ensure you have Go 1.24+ installed

## Building the Application

### Build for Local Development

Build the binaries with CGO enabled (for local development):

```bash
cd /home/xhan/projects/works/go/src/xhanio/framingo/example
gopro build binary -e local
```

Two binaries are produced under `bin/`:
- `bin/exampleapp` - daemon (HTTP API server)
- `bin/examplecli` - client CLI for the API

### Build for Production

Build a static binary for production deployment:

```bash
gopro build binary -e prod
```

## Running the Application

### Run with Configuration File

```bash
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml
```

Or with environment variables loaded:

```bash
# Load environment variables
export $(cat env/local/config/exampleapp/secret.env | xargs)

# Run the application
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml
```

### Test the API

Once the application is running:

```bash
# POST /api/v1/example/helloworld (requires authnuser middleware - login first via examplecli)
curl -X POST http://localhost:8080/api/v1/example/helloworld \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello World"}'

# Expected response:
# {
#   "id": 1,
#   "message": "Hello World",
#   "created_at": "2024-01-15T10:30:00Z",
#   "updated_at": "2024-01-15T10:30:00Z"
# }
```

### Check Version

```bash
./bin/exampleapp version
./bin/examplecli version
```

## Using the CLI

`examplecli` talks to the daemon's HTTP API. Credentials are persisted at `~/.example`.

```bash
# Login (prompts for password, default username: admin)
./bin/examplecli -e http://localhost:8080 login

# Call the helloworld endpoint
./bin/examplecli -e http://localhost:8080 helloworld "Hello World"

# Logout
./bin/examplecli -e http://localhost:8080 logout

# Generate CA + server cert/key in CWD
./bin/examplecli certutil -p framingo --domain localhost --ip 127.0.0.1
```

Global flags: `-e/--endpoint`, `-v/--verbose`.

## Development Workflow

### 1. Make Code Changes

Edit files in `pkg/` directory:
- `pkg/services/example/` - Business logic
- `pkg/routers/example/` - HTTP handlers
- `pkg/middlewares/example/` - Request processing
- `pkg/components/server/example/` - Server orchestration

### 2. Rebuild

```bash
gopro build binary -e local
```

### 3. Test

```bash
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml
./bin/examplecli -e http://localhost:8080 helloworld "Test"
```

## Docker Deployment

### Build Docker Image

```bash
# First build the binary
gopro build binary -e prod

# Then build the image
gopro build image
```

### Run in Docker

```bash
docker run -p 8080:8080 -p 6060:6060 \
  -v $(pwd)/env/local/config/exampleapp/config.yaml:/etc/framingo-example/config.yaml \
  localhost:5000/exampleapp:latest
```

## Kubernetes Deployment

### Generate Kubernetes Manifests

```bash
gopro generate kubernetes -e local
```

Generated files will be in: `dist/local/kubernetes/exampleapp/`

### Deploy to Kubernetes

```bash
kubectl apply -f dist/local/kubernetes/exampleapp/
```

### Check Deployment

```bash
# Check pods
kubectl get pods -l app=framingo-example

# Check service
kubectl get svc framingo-example

# View logs
kubectl logs -l app=framingo-example -f
```

### Access the Service

```bash
# Port forward to access locally
kubectl port-forward svc/framingo-example 8080:8080

# Test the API (login first; see "Using the CLI" above)
./bin/examplecli -e http://localhost:8080 helloworld "Hello"
```

## Configuration Management

### Generate Configuration Files

```bash
# Generate config for local environment
gopro generate config -e local

# Generate config for production
gopro generate config -e prod
```

Generated files will be in:
- Local: `dist/local/config/exampleapp/`
- Prod: `dist/prod/config/exampleapp/`

## Debugging

### Enable Profiling

The application runs pprof on port 6060 (configured in config.yaml):

```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Web interface
open http://localhost:6060/debug/pprof/
```

### Signal Handling

```bash
# Get PID
PID=$(pgrep exampleapp)

# Print service info
kill -USR1 $PID

# Print stack traces
kill -USR2 $PID

# Graceful shutdown
kill -INT $PID
```

## Directory Structure

```
example/
├── project.yaml                 # GoPro configuration
├── build/                       # Build sources
│   ├── binary/
│   │   ├── exampleapp/
│   │   │   └── main.go         # Daemon entry point
│   │   └── examplecli/
│   │       └── main.go         # CLI entry point
│   └── image/
│       └── exampleapp/
│           └── Dockerfile      # Docker image definition
├── env/                        # Environment configurations
│   └── local/
│       ├── config/
│       │   └── exampleapp/
│       │       ├── config.yaml # Application config
│       │       ├── secret.env  # Environment variables
│       │       └── migrations/ # SQL migrations (000_create_system_tables, 001_create_helloworld_table)
│       └── kubernetes/
│           └── exampleapp/
│               ├── deployment.yaml
│               ├── service.yaml
│               └── configmap.yaml
├── pkg/                        # Application code
│   ├── components/
│   ├── services/               # Business logic (validation, message dispatch, entity conversion)
│   │   └── repository/         # DB access layer (one file per domain)
│   ├── routers/
│   ├── middlewares/
│   ├── types/                  # api/, entity/, orm/, repo/ (interface definitions)
│   └── utils/
├── bin/                        # Built binaries (generated)
└── dist/                       # Generated configs (generated)
```

## Troubleshooting

### Binary Build Fails

Check Go module dependencies:
```bash
go mod download
go mod tidy
```

### Application Won't Start

Check configuration file path:
```bash
ls -la env/local/config/exampleapp/config.yaml
```

Verify database connection (if using DB):
```bash
# Check PostgreSQL is running
psql -h localhost -U framingo -d framingo_example
```

### API Returns 404

Verify the correct endpoint and method:
```bash
# Correct endpoint: POST /api/v1/example/helloworld (prefix configurable via api.http.prefix)
# Note: requires the `authnuser` middleware - use `examplecli login` first

# Check registered routes in logs
grep "Registering route" /var/log/framingo-example/app.log
```

## Next Steps

1. Explore the example implementations in `pkg/`
2. Modify the example service to add your business logic
3. Create new routers, services, and middlewares following the patterns
4. Update configuration in `env/` for your environment
5. Deploy to your target environment

For more details, see the main [README.md](README.md).
