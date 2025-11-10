# Quick Start Guide

This guide shows how to quickly build and run the Framingo example application using GoPro.

## Prerequisites

1. Install GoPro:
   ```bash
   go install github.com/xhanio/gopro@latest
   ```

2. Ensure you have Go 1.21+ installed

## Building the Application

### Build for Local Development

Build the binary with CGO enabled (for local development):

```bash
cd /home/xhan/projects/works/go/src/xhanio/framingo/example
gopro build binary -e local
```

The binary will be created at: `bin/example-app`

### Build for Production

Build a static binary for production deployment:

```bash
gopro build binary -e prod
```

## Running the Application

### Run with Configuration File

```bash
./bin/example-app daemon -c env/local/config/example-app/config.yaml
```

Or with environment variables loaded:

```bash
# Load environment variables
export $(cat env/local/config/example-app/secret.env | xargs)

# Run the application
./bin/example-app daemon -c env/local/config/example-app/config.yaml
```

### Test the API

Once the application is running:

```bash
# Test the example endpoint
curl http://localhost:8080/api/v1/demo/example

# Expected response: Good
```

### Check Version

```bash
./bin/example-app version
```

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
./bin/example-app daemon -c env/local/config/example-app/config.yaml
curl http://localhost:8080/api/v1/demo/example
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
  -v $(pwd)/env/local/config/example-app/config.yaml:/etc/framingo-example/config.yaml \
  localhost:5000/example-app:latest
```

## Kubernetes Deployment

### Generate Kubernetes Manifests

```bash
gopro generate kubernetes -e local
```

Generated files will be in: `dist/local/kubernetes/example-app/`

### Deploy to Kubernetes

```bash
kubectl apply -f dist/local/kubernetes/example-app/
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

# Test the API
curl http://localhost:8080/api/v1/demo/example
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
- Local: `dist/local/config/example-app/`
- Prod: `dist/prod/config/example-app/`

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
PID=$(pgrep example-app)

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
│   │   └── example-app/
│   │       └── main.go         # Application entry point
│   └── image/
│       └── example-app/
│           └── Dockerfile      # Docker image definition
├── env/                        # Environment configurations
│   └── local/
│       ├── config/
│       │   └── example-app/
│       │       ├── config.yaml # Application config
│       │       └── secret.env  # Environment variables
│       └── kubernetes/
│           └── example-app/
│               ├── deployment.yaml
│               ├── service.yaml
│               └── configmap.yaml
├── pkg/                        # Application code
│   ├── components/
│   ├── services/
│   ├── routers/
│   ├── middlewares/
│   ├── types/
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
ls -la env/local/config/example-app/config.yaml
```

Verify database connection (if using DB):
```bash
# Check PostgreSQL is running
psql -h localhost -U framingo -d framingo_example
```

### API Returns 404

Verify the correct endpoint:
```bash
# Correct endpoint
curl http://localhost:8080/api/v1/demo/example

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
