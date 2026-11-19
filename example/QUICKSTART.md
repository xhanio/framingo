# Quick Start Guide

This guide shows how to build, run, and (most importantly) **use this `example/` folder as the starting code base for your own Framingo project**.

The example is structured as a self-contained module (`github.com/xhanio/framingo/example`) with its own `go.mod`, GoPro `project.yaml`, build templates, environment configs, and Kubernetes manifests. Everything you need to ship a production service is wired up — auth, RBAC, database, migrations, pubsub + WebSocket message bus, CLI client, Docker image, and K8s manifests. Fork it, rename it, gut what you don't need, add your domain.

## Use This Folder as Your Starting Template

### What You Get Out of the Box

| Layer | What's wired up |
|---|---|
| **Binaries** | `exampleapp` (`daemon`, `migrate`, `version`) + `examplecli` (HTTP client CLI) |
| **Supervisor** | Topologically-sorted lifecycle, a bounded stop (`WithStopPolicy(20s)`); a monitor (`supervisor.monitor.*`: every 10s, 3 restarts) keeps `/readyz` live and restarts a service whose liveness fails, `/healthz` failing once those restarts are spent. Signal handling is app code, in `pkg/components/server/example/signal.go` — the framework installs no handlers |
| **Persistence** | PostgreSQL via GORM, migrations (on start locally, by a one-off `exampleapp migrate` Job in prod), connection pooling, transactions |
| **AuthN/AuthZ** | User login (with optional LDAP/API-token hooks), session cookies, role-based authorization. `pkg/middlewares/authnagent/` sketches mTLS agent auth but is **not registered and not working** — its CN parsing rejects the certs this system issues, and it verifies only the signature, not expiry or key usage |
| **Messaging** | Pub/sub primitive + message bus + WebSocket stream endpoint (`/api/v1/messages/stream`) |
| **HTTP API** | Echo-based server with declarative YAML routing, CORS, throttling, deflate compression; a feature-flag middleware (`pkg/middlewares/feature/`) ships unregistered |
| **System services** | User, role, organization, certificate (PKI) management |
| **Build & deploy** | GoPro-driven binary, Docker image, docker-compose, and Kubernetes manifests |
| **Observability** | Structured logging with rotation, OpenTelemetry tracing (HTTP + GORM spans) and metrics (stdout exporter locally), `/healthz` + `/readyz` on the health listener (`:8081`), pprof on `:6060`, debug dump on `SIGUSR1`, stack dump on `SIGUSR2` |

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
#    - FRAMINGO_EXAMPLE_           →   MYAPP_          (secret.env*: the env prefix follows the product)
find . -type f \( -name '*.yaml' -o -name '*.go' -o -name 'Dockerfile' -o -name 'secret.env*' \) \
  -exec sed -i 's/framingo-example/myapp/g; s/exampleapp/myappd/g; s/examplecli/myapp/g; s/FRAMINGO_EXAMPLE_/MYAPP_/g' {} +

# 4. Rename the per-binary directories and configs
mv build/binary/exampleapp build/binary/myappd
mv build/binary/examplecli build/binary/myapp
mv build/image/exampleapp build/image/myappd
mv env/default/config/exampleapp env/default/config/myappd
mv env/local/config/exampleapp env/local/config/myappd
mv env/prod/config/exampleapp env/prod/config/myappd
mv env/prod/kubernetes/exampleapp env/prod/kubernetes/myappd

# 5. Verify it still builds
go mod tidy
gopro build binary -e local
```

> The `sed` recipes above are starting points — review the diff before committing. Inside `pkg/` the rename of `pkg/services/example/` and `pkg/routers/example/` to your own domain is a separate step you'll do as you replace the demo HelloWorld feature with your real business logic.

### What to Keep, What to Rip Out

| Keep | Why |
|---|---|
| `pkg/components/server/` | The supervisor wiring pattern; just register/unregister services |
| `pkg/components/cmd/app/` | The Cobra `daemon` / `migrate` / `version` skeleton |
| `pkg/services/repository/` | One-file-per-domain GORM repo pattern; add your own |
| `pkg/types/{api,entity,model,orm,repo}/` | The strict type-separation layout |
| Build/env/K8s templates | GoPro builds, Docker image, manifests |

| Optional — remove if you don't need it | Where |
|---|---|
| RBAC + user/role/org/cert system services | `pkg/services/system/`, `pkg/routers/{auth,user,role,certificate}/`, `pkg/middlewares/{authnuser,authnagent,authz}/`, `pkg/types/{rbac,preset}/` |
| WebSocket message bus stream | `pkg/routers/messagebus/` and its `messagebusRouter.New` line in `pkg/components/server/example/api.go`, plus the CLI's `stream` (`pkg/components/cmd/cli/messagebus.go`) |
| Feature-flag middleware | `pkg/middlewares/feature/`, `pkg/types/rbac/feature.go` |
| Deflate compression middleware | `pkg/middlewares/deflate/` |
| Demo HelloWorld feature | `pkg/services/example/`, `pkg/routers/example/`, `pkg/types/message/`, plus the `001_create_helloworld_table.*.sql` migration |
| `examplecli` binary | `build/binary/examplecli/`, `pkg/components/cmd/cli/`, `pkg/components/client/` |

The supervisor's dependency resolution means you can delete a service entirely: as long as nothing else lists it in `Dependencies()`, removing it from `initServices()` and the `Register(...)` calls is enough.

---

## What Is Framingo Here, and What Is GoPro

This folder is two things layered together, and it helps to know which is which
before you start deleting parts of it.

| | Owned by | Lives in |
|---|---|---|
| Services, supervisor wiring, routers, middlewares, type layout, the shape of `config.yaml` | **framingo** | `pkg/` |
| Building binaries, per-environment config generation, Docker images, K8s manifests | **[GoPro](https://github.com/xhanio/gopro)** | `project.yaml`, `build/`, `env/`, `dist/` |

So anything below involving `project.yaml`, `-e <env>`, `dist/`, `gopro build`
or `gopro generate` is GoPro's, not framingo's — consult GoPro's own docs for
those, since this guide only covers the paths this template uses.

**framingo does not require GoPro.** The entry points under `build/binary/` are
ordinary `main` packages:

```bash
CGO_ENABLED=1 go build -o bin/exampleapp ./build/binary/exampleapp
```

Two things GoPro does for you that you then have to handle yourself:

1. **`dist/` is generated and gitignored**, so a fresh clone does not have it —
   yet `config.yaml` sets `db.migration.dir` to `./dist/local/config/exampleapp/migrations`.
   Either run `gopro generate config -e local` once, or repoint that key at
   `./env/default/config/exampleapp/migrations`, where the migrations every
   environment shares live.
2. **Build metadata is injected at link time** into `pkg/types/info` (product
   name, version, git commit). Without it `info.ProductName` is empty, which
   leaves the env-var prefix blank — so overrides are `DB_SOURCE_HOST` rather than
   `FRAMINGO_EXAMPLE_DB_SOURCE_HOST` — and leaves the product name out of the
   system CAs' common names. Harmless for local poking; supply your own `-ldflags` if you
   care.

## Prerequisites

1. **Go 1.25.8+** — this module's own `go` directive (the framework itself needs 1.25). On an older toolchain the default `GOTOOLCHAIN=auto` fetches it; `GOTOOLCHAIN=local` will fail.
2. **A C toolchain** — the SQLite driver is blank-imported, so builds run with `CGO_ENABLED=1`.
3. Install GoPro: `go install github.com/xhanio/gopro@latest`
4. (For the default config) a local PostgreSQL with database `framingo_example`, user `framingo`, password `framingo_dev`

## Building

```bash
cd example/                          # or your forked project root
gopro build binary -e local          # CGO-enabled local dev build
gopro generate config -e local       # → dist/local/config/exampleapp/: config.yaml + migrations
```

The second step matters: the local `config.yaml` points `db.migration.dir`
at `./dist/local/config/exampleapp/migrations`, relative to the project
root, and `gopro build binary` does not render it.

`project.yaml` defines two environments: `local`, which runs under docker
compose, and `prod`, which deploys to Kubernetes. `env/default/` holds what
both share — the migrations — and each environment's tree renders on top of
it. To add another, give it an entry under `env:` pointing
`config_src`/`config_tgt` at a matching `env/<name>/` tree.

Produces:
- `bin/exampleapp` — daemon (HTTP API server)
- `bin/examplecli` — HTTP client CLI

## Running

```bash
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml

# Or with env vars layered on top - each overrides the config key it names
set -a; . env/local/config/exampleapp/secret.env; set +a
./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml
```

The daemon migrates on start, since the local config sets `db.migration`.
To migrate without serving, as prod's Job does:
`./bin/exampleapp migrate -c env/local/config/exampleapp/config.yaml`.

Any config key can be overridden this way: the product prefix, then the key
with its dots as underscores — `db.source.host` is
`FRAMINGO_EXAMPLE_DB_SOURCE_HOST` — and the environment wins over the file.

### Exercise the HelloWorld Endpoint

The endpoint is protected by `authnuser`, so log in first. `-e` is the API
base, prefix included; it defaults to `http://127.0.0.1:8080/api/v1`:

```bash
./bin/examplecli -e http://localhost:8080/api/v1 login    # -u defaults to admin; prompts for the password (admin)
./bin/examplecli -e http://localhost:8080/api/v1 helloworld "Hello"
# {"id":1,"message":"Hello","created_at":"...","updated_at":"..."}   (the greeting only goes to the server log)
./bin/examplecli -e http://localhost:8080/api/v1 logout
```

Equivalent curl. `login` persists the session id to `~/.example` as
`{"session_id":"..."}`, and the CLI sends it in the `X-SESSION-ID` header
rather than as a cookie:

```bash
curl -X POST http://localhost:8080/api/v1/example/helloworld \
  -H 'Content-Type: application/json' \
  -H "X-SESSION-ID: $(jq -r .session_id ~/.example)" \
  -d '{"message":"Hello"}'
```

### Other CLI Commands

```bash
./bin/examplecli -e http://localhost:8080/api/v1 stream   # subscribe to the WS stream; Ctrl-C closes it cleanly
./bin/examplecli certutil -p framingo --domain localhost --ip 127.0.0.1   # writes ca.crt/.key + server.crt/.key here
./bin/examplecli version
./bin/exampleapp version
```

Global flags: `-e/--endpoint`, `-v/--verbose` (request debug output).

## Development Workflow

1. Edit code under `pkg/`
2. `gopro build binary -e local` (and `gopro generate config -e local` after touching `env/`)
3. Re-run `./bin/exampleapp daemon -c env/local/config/exampleapp/config.yaml`
4. Exercise via `./bin/examplecli` or `curl`

## Local: Docker Compose

The whole stack — postgres and the daemon — in containers:

```bash
gopro build binary -e local
gopro generate config -e local          # the image copies the rendered config and migrations
gopro build image -e local -f '^exampleapp$'
gopro generate docker-compose -e local  # → dist/local/docker-compose.yaml

cd dist/local && docker compose up -d
curl http://localhost:8081/readyz       # {"ready":true}
```

The compose file takes the database credentials from
`env/local/config/exampleapp/secret.env` at generate time, for postgres and
the daemon alike. Locally the single daemon migrates on start, from the
migrations the image carries (`FRAMINGO_EXAMPLE_DB_MIGRATION_DIR=/app/migrations`).

## Prod: Kubernetes

```bash
cp env/prod/config/exampleapp/secret.env.example env/prod/config/exampleapp/secret.env
# fill in the database credentials - secret.env is gitignored; in CI, provision it from
# your secrets manager and discard it after generating

gopro build binary -e prod
gopro generate config -e prod
gopro build image -e prod -f '^exampleapp$' --push   # set image_prefix for prod in project.yaml first
gopro generate kubernetes -e prod       # → dist/prod/kubernetes/exampleapp/

cd dist/prod/kubernetes/exampleapp
kubectl apply -f configmap.yaml         # ConfigMap and Secret, which the Job reads too
kubectl apply -f migrate-job.yaml
kubectl wait --for=condition=complete --timeout=5m job -l component=migrate
kubectl apply -f service.yaml -f deployment.yaml
```

In prod the daemon never touches the schema: its config sets no
`db.migration`, and the migrate Job — `exampleapp migrate`, run with the
release's own image and the migrations it carries — applies it once per
release, so replicas never race each other over it. The Job's name carries the
version (`framingo-example-migrate-v0-8-1`), since a Job cannot be changed
once created. The probes hit `/healthz` and `/readyz` on the health listener
(`api.health`, port 8081); `db_host` in the ConfigMap points at a Service
named `postgres` — point it at your database.

## Configuration Generation

```bash
gopro generate config -e local          # → dist/local/config/exampleapp/
# ...and one per environment you add under `env:` in project.yaml
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
kill -HUP  $PID    # restart every service (ignored while shutting down)
kill -INT  $PID    # graceful shutdown; a second INT/TERM forces it, a third exits at once
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
├── env/
│   ├── default/config/exampleapp/migrations/ # shared by every env: 000_create_system_tables, 001_create_helloworld_table
│   ├── local/                                # docker compose
│   │   ├── config/exampleapp/
│   │   │   ├── config.yaml                   # log, db, tracing, metrics, api, example, pprof
│   │   │   └── secret.env                    # development credentials, env-var overrides
│   │   └── docker-compose/template.docker-compose.yaml   # postgres + the daemon
│   └── prod/                                 # Kubernetes
│       ├── config/exampleapp/
│       │   ├── config.yaml                   # the production config the ConfigMap carries
│       │   └── secret.env.example            # copy to secret.env (gitignored) and fill in
│       └── kubernetes/exampleapp/            # deployment, service, configmap + secret, migrate job
├── dist/                                     # generated by `gopro generate`
├── bin/                                      # built binaries
└── pkg/
    ├── components/
    │   ├── client/example/                   # HTTP client SDK used by examplecli
    │   ├── cmd/
    │   │   ├── app/                          # daemon Cobra commands (root, daemon, migrate, version)
    │   │   └── cli/                          # CLI commands (auth, cert, example, messagebus)
    │   └── server/example/                   # supervisor wiring (model, manager, lifecycle, config, service, api, signal, migrate)
    ├── services/
    │   ├── example/                          # demo HelloWorld business service
    │   ├── repository/                       # one file per domain (helloworld, user, role, ...)
    │   └── system/
    │       ├── platform/                     # the db/meter/pubsub/messagebus bundle services take first
    │       ├── auth/                         # user / LDAP / API-token authentication
    │       ├── user/                         # user CRUD
    │       ├── role/                         # role definitions
    │       ├── organization/                 # multi-tenant orgs
    │       └── certificate/                  # mTLS / PKI cert management
    ├── routers/                              # auth, certificate, example, health, messagebus, role, user
    ├── middlewares/                          # authnagent, authnuser, authz, cors, deflate, feature, throttle
    ├── types/                                # api, entity, message, model, orm, preset, rbac, repo
    └── utils/infra/                          # process-wide facts set at startup: env prefix, config dir, start time
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

- Endpoint: `POST /api/v1/example/helloworld` (prefix from `api.http.prefix`); `examplecli -e` must carry the prefix too
- Requires `authnuser` — log in first via `examplecli login`
- Check registered routers (logged at debug, which local's `log.level: -1` is): `grep "registered router" <log.file>`

## Next Steps

1. Fork the folder per the [template steps above](#forking-the-example-into-your-own-repo)
2. Strip out system services you don't need (see [What to Keep, What to Rip Out](#what-to-keep-what-to-rip-out))
3. Replace `pkg/services/example/` and `pkg/routers/example/` with your own domain
4. Add migrations under `env/default/config/<your-app>/migrations/`
5. Adjust `env/prod/kubernetes/<your-app>/` for your cluster, and set prod's `image_prefix` in `project.yaml`

For framework concepts and core packages, see the [main README](../README.md).
