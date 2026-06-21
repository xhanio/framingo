# Quick Start Guide

This guide shows how to build, run, and (most importantly) **use this `example/` folder as the starting code base for your own Framingo project**.

The example is structured as a self-contained module (`github.com/xhanio/framingo/example`) with its own `go.mod`, GoPro `project.yaml`, build templates, environment configs, and Kubernetes manifests. Everything you need to ship a production service is wired up — auth, RBAC, database, migrations, pubsub + WebSocket message bus, CLI client, Docker image, and K8s manifests. Fork it, rename it, gut what you don't need, add your domain.

## Use This Folder as Your Starting Template

### What You Get Out of the Box

| Layer | What's wired up |
|---|---|
| **Binaries** | `exampleapp` (daemon) + `examplecli` (HTTP client CLI) |
| **Supervisor** | Topologically-sorted lifecycle, signal handling, auto-restart on liveness failure |
| **Persistence** | PostgreSQL via GORM, migrations, connection pooling, transactions |
| **AuthN/AuthZ** | User login (with optional LDAP/API-token hooks), session cookies, role-based authorization, mTLS agent auth |
| **Messaging** | Pub/sub primitive + message bus + WebSocket stream endpoint (`/api/v1/messages/stream`) |
| **HTTP API** | Echo-based server with declarative YAML routing, throttling, deflate compression, feature flags |
| **System services** | User, role, organization, certificate (PKI) management |
| **Build & deploy** | GoPro-driven binary, Docker image, docker-compose, and Kubernetes manifests |
| **Observability** | Structured logging with rotation, pprof on `:6060`, debug dump on `SIGUSR1`, stack dump on `SIGUSR2` |

### Forking the Example into Your Own Repo

Replace `myapp` and `MYORG` below with your own names.

```bash
# 1. Copy the example into a new repo
cp -r example/ ~/projects/myapp
cd ~/projects/myapp

# 2. Re-initialize the Go module
rm -rf .git
git init
go mod edit -module github.com/MYORG/myapp
find . -type f -name '*.go' -exec sed -i 's|github.com/xhanio/framingo/example|github.com/MYORG/myapp|g' {} +

# 3. Rename the product and binaries (search/replace across the tree)
#    - product: framingo-example   →   myapp           (project.yaml)
#    - exampleapp / examplecli     →   myappd / myapp  (project.yaml, build/, env/)
#    - framingo-example            →   myapp           (Dockerfile, K8s manifests, log paths)
find . -type f \( -name '*.yaml' -o -name '*.go' -o -name 'Dockerfile' \) \
  -exec sed -i 's/framingo-example/myapp/g; s/exampleapp/myappd/g; s/examplecli/myapp/g' {} +

# 4. Rename the per-binary directories and configs
mv build/binary/exampleapp build/binary/myappd
mv build/binary/examplecli build/binary/myapp
mv build/image/exampleapp build/image/myappd
mv env/local/config/exampleapp env/local/config/myappd
mv env/local/kubernetes/exampleapp env/local/kubernetes/myappd

# 5. Verify it still builds
go mod tidy
gopro build binary -e local
```

> The `sed` recipes above are starting points — review the diff before committing. Inside `pkg/` the rename of `pkg/services/example/` and `pkg/routers/example/` to your own domain is a separate step you'll do as you replace the demo HelloWorld feature with your real business logic.

### What to Keep, What to Rip Out

| Keep | Why |
|---|---|
| `pkg/components/server/` | The supervisor wiring pattern; just register/unregister services |
| `pkg/components/cmd/app/` | The Cobra `daemon` / `version` skeleton |
| `pkg/services/repository/` | One-file-per-domain GORM repo pattern; add your own |
| `pkg/types/{api,entity,model,orm,repo}/` | The strict type-separation layout |
| Build/env/K8s templates | GoPro builds, Docker image, manifests |

| Optional — remove if you don't need it | Where |
|---|---|
| RBAC + user/role/org/cert system services | `pkg/services/system/`, `pkg/routers/{auth,user,role,certificate}/`, `pkg/middlewares/{authnuser,authnagent,authz}/`, `pkg/types/{rbac,preset}/` |
| WebSocket message bus stream | `pkg/routers/messagebus/`, drop `messagebus` from `pkg/components/server/example/service.go` |
| Feature-flag middleware | `pkg/middlewares/feature/`, `pkg/types/rbac/feature.go` |
| Deflate compression middleware | `pkg/middlewares/deflate/` |
| Demo HelloWorld feature | `pkg/services/example/`, `pkg/routers/example/`, `pkg/types/message/`, plus the `001_create_helloworld_table.*.sql` migration |
| `examplecli` binary | `build/binary/examplecli/`, `pkg/components/cmd/cli/`, `pkg/components/client/` |

The supervisor's dependency resolution means you can delete a service entirely: as long as nothing else lists it in `Dependencies()`, removing it from `initServices()` and the `Register(...)` calls is enough.

---

## Prerequisites

1. Go 1.24+
2. Install GoPro: `go install github.com/xhanio/gopro@latest`
3. (For the default config) a local PostgreSQL with database `framingo_example`, user `framingo`, password `framingo_dev`

## Building

```bash
cd example/                          # or your forked project root
gopro build binary -e local          # CGO-enabled local dev build
# gopro build binary -e prod         # static binary for production
```

Produces:
- `bin/exampleapp` — daemon (HTTP API server)
- `bin/examplecli` — HTTP client CLI

## Running

```bash
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml

# Or with env vars layered on top
export $(cat env/local/config/exampleapp/secret.env | xargs)
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml
```

### Exercise the HelloWorld Endpoint

The endpoint is protected by `authnuser`, so log in first:

```bash
./bin/examplecli -e http://localhost:8080 login           # default: admin / admin
./bin/examplecli -e http://localhost:8080 helloworld "Hello"
# {"id":1,"message":"hello world!!! Hello","created_at":"...","updated_at":"..."}
./bin/examplecli -e http://localhost:8080 logout
```

Equivalent curl (after login persists a cookie at `~/.example`):

```bash
curl -X POST http://localhost:8080/api/v1/example/helloworld \
  -H 'Content-Type: application/json' \
  --cookie "$(cat ~/.example | jq -r .cookie)" \
  -d '{"message":"Hello"}'
```

### Other CLI Commands

```bash
./bin/examplecli -e http://localhost:8080 messagebus stream    # subscribe to WS stream
./bin/examplecli certutil -p framingo --domain localhost --ip 127.0.0.1
./bin/examplecli version
./bin/exampleapp version
```

Global flags: `-e/--endpoint`, `-v/--verbose`.

## Development Workflow

1. Edit code under `pkg/`
2. `gopro build binary -e local`
3. Re-run `./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml`
4. Exercise via `./bin/examplecli` or `curl`

## Docker

```bash
gopro build binary -e prod
gopro build image

docker run -p 8080:8080 -p 6060:6060 \
  -v $(pwd)/env/local/config/exampleapp/config.yaml:/etc/framingo-example/config.yaml \
  localhost:5000/exampleapp:latest
```

## Kubernetes

```bash
gopro generate kubernetes -e local      # → dist/local/kubernetes/exampleapp/
kubectl apply -f dist/local/kubernetes/exampleapp/

kubectl get pods -l app=framingo-example
kubectl logs  -l app=framingo-example -f
kubectl port-forward svc/framingo-example 8080:8080

./bin/examplecli -e http://localhost:8080 login
./bin/examplecli -e http://localhost:8080 helloworld "Hello"
```

## Configuration Generation

```bash
gopro generate config -e local          # → dist/local/config/exampleapp/
gopro generate config -e prod           # → dist/prod/config/exampleapp/
```

## Debugging

### pprof (port `6060`, from `config.yaml`)

```bash
go tool pprof http://localhost:6060/debug/pprof/profile        # CPU
go tool pprof http://localhost:6060/debug/pprof/heap           # heap
go tool pprof http://localhost:6060/debug/pprof/goroutine      # goroutines
open http://localhost:6060/debug/pprof/                        # web UI
```

### Signals

```bash
PID=$(pgrep exampleapp)
kill -USR1 $PID    # dump per-service info to stdout (Debuggable.Info)
kill -USR2 $PID    # dump goroutine stack traces
kill -INT  $PID    # graceful shutdown
```

## Directory Structure

```
example/
├── project.yaml                              # GoPro project config (product, binaries, images)
├── go.mod                                    # module github.com/xhanio/framingo/example
├── build/
│   ├── binary/
│   │   ├── exampleapp/main.go                # daemon entry point
│   │   └── examplecli/main.go                # CLI entry point
│   └── image/exampleapp/Dockerfile           # docker image definition
├── env/local/
│   ├── config/exampleapp/
│   │   ├── config.yaml                       # log, db, api, pprof
│   │   ├── secret.env                        # env-var overrides for DB creds, etc.
│   │   └── migrations/                       # 000_create_system_tables, 001_create_helloworld_table
│   ├── docker-compose/docker-compose.yaml
│   └── kubernetes/exampleapp/                # deployment, service, configmap
├── dist/                                     # generated by `gopro generate`
├── bin/                                      # built binaries
└── pkg/
    ├── components/
    │   ├── client/example/                   # HTTP client SDK used by examplecli
    │   ├── cmd/
    │   │   ├── app/                          # daemon Cobra commands (root, daemon, version)
    │   │   └── cli/                          # CLI commands (auth, cert, example, messagebus)
    │   └── server/example/                   # supervisor wiring (model, manager, lifecycle, config, service, api, signal)
    ├── services/
    │   ├── example/                          # demo HelloWorld business service
    │   ├── repository/                       # one file per domain (helloworld, user, role, ...)
    │   └── system/
    │       ├── auth/                         # user / LDAP / API-token authentication
    │       ├── user/                         # user CRUD
    │       ├── role/                         # role definitions
    │       ├── organization/                 # multi-tenant orgs
    │       └── certificate/                  # mTLS / PKI cert management
    ├── routers/                              # auth, certificate, example, messagebus, role, user
    ├── middlewares/                          # authnagent, authnuser, authz, deflate, feature
    ├── types/                                # api, entity, infra, message, model, orm, preset, rbac, repo
    └── utils/infra/                          # local infra helpers
```

## Troubleshooting

### Binary build fails

```bash
go mod download
go mod tidy
```

### App won't start

```bash
ls -la env/local/config/exampleapp/config.yaml
psql -h localhost -U framingo -d framingo_example          # verify DB reachable
```

### API returns 404 or 401

- Endpoint: `POST /api/v1/example/helloworld` (prefix from `api.http.prefix`)
- Requires `authnuser` — log in first via `examplecli login`
- Check registered routes: `grep "Registering route" /var/log/framingo-example/app.log`

## Next Steps

1. Fork the folder per the [template steps above](#forking-the-example-into-your-own-repo)
2. Strip out system services you don't need (see [What to Keep, What to Rip Out](#what-to-keep-what-to-rip-out))
3. Replace `pkg/services/example/` and `pkg/routers/example/` with your own domain
4. Add migrations under `env/local/config/<your-app>/migrations/`
5. Adjust `env/local/kubernetes/<your-app>/` for your cluster

For framework concepts and core packages, see the [main README](../README.md).
