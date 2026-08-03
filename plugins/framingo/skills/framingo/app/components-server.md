# Server Component — The Application Daemon

`pkg/components/server/<app>/` is the heart of the wiring: it builds the
service graph, hands it to the supervisor, registers routers and middlewares,
and owns signals. All server implementations MUST follow this file structure —
the reference implementation is
[`example/pkg/components/server/example/`](https://github.com/xhanio/framingo/tree/main/example/pkg/components/server/example)
in the framework repo. Each file has a specific responsibility:

```
components/server/myapp/
├── model.go     # Server interface definition (Named + Daemon + Initializable + Debuggable)
├── manager.go   # Main struct, New(), Name() — struct fields and construction only
├── lifecycle.go # Init(), Start(), Stop(), Info() — orchestrates everything in order
├── config.go    # Viper config creation (newConfig) and loading (initConfig)
├── service.go   # initServices() — creates ALL service instances in layered order
├── api.go       # initAPI() — registers middlewares and routers with the API server
└── signal.go    # listenSignals() — OS signal handling (SIGINT, SIGTERM, SIGUSR1, SIGUSR2)
```

The daemon binary constructs this component and runs it — see
[components-cmd.md](components-cmd.md).

## `model.go` — Server Interface

```go
type Server interface {
    common.Named
    common.Daemon        // Start(ctx) / Stop(wait)
    common.Initializable // Init(ctx)
    common.Debuggable    // Info(w, debug)
}
```

## `manager.go` — Struct and Construction

`manager.go` holds the struct definition and the `New()`/`Name()` methods only. Lifecycle methods live in `lifecycle.go`.

```go
type manager struct {
    name   string
    config *viper.Viper
    log    log.Logger

    // infra services
    db         db.Manager
    pubsub     pubsub.Manager
    messagebus messagebus.Manager

    // business services
    userSvc user.Manager

    // api services
    api server.Manager

    // service controller
    services supervisor.Manager
    ctx      context.Context
    cancel   context.CancelFunc
}

func New(configPath string) Server {
    return &manager{config: newConfig(configPath)}
}
```

## `lifecycle.go` — Orchestration

Implements `Init`, `Start`, `Stop`, `Info`. `Init()` calls `initConfig()` → `initServices()` → registers services with supervisor in dependency layers → `TopoSort()` → registers `m.api` AFTER the sort so it starts last → registers all services with the messagebus → `services.Init()` → `initAPI()`. `Start()` starts all services and blocks on `<-ctx.Done()`.

### Registration Order in `Init()`

```go
func (m *manager) Init(ctx context.Context) error {
    m.initConfig()
    m.initServices()

    // Register in dependency layers
    m.services.Register(m.db)                                // basic infra
    m.services.Register(m.pubsub, m.messagebus, m.userSvc)   // system + business
    m.services.TopoSort()                                    // resolve dependency order
    m.services.Register(m.api)                               // API registered AFTER sort to ensure it starts last

    // Register all services with the messagebus. Services that don't implement
    // MessageHandler / RawMessageHandler are skipped automatically.
    for _, svc := range m.services.Services() {
        m.messagebus.Register(svc)
    }

    m.services.Init(ctx)                                     // init all services in dependency order
    m.initAPI()                                              // wire routes after services are initialized
    return nil
}
```

## `service.go` — Service Creation (Layered Order)

Services MUST be created in this layered order:

```go
func (m *manager) initServices() error {
    // 1. Logger — first, everything depends on it
    m.log = log.New(...)

    // 2. Supervisor (service controller)
    m.services = supervisor.New(m.config, supervisor.WithLogger(m.log))

    // 3. Infra services: database, pubsub, messagebus
    //    NOTE: blank-import the db driver subpackage(s) this binary supports —
    //    e.g. `_ "github.com/xhanio/framingo/pkg/services/db/drivers/postgres"` —
    //    at the top of this file. The core `db` package no longer imports any
    //    concrete driver, so unimported engines fail at Init with
    //    "driver not registered".
    m.db = db.New(db.WithType(...), db.WithDataSource(...), db.WithLogger(m.log))
    m.pubsub = pubsub.New(driver.NewMemory(m.log), pubsub.WithLogger(m.log))
    m.messagebus = messagebus.New(m.pubsub, messagebus.WithLogger(m.log))

    // 4. Business services
    m.userSvc = user.New(m.db, user.WithLogger(m.log))

    // 5. API server (created last, started last)
    m.api = server.New(server.WithLogger(m.log))
    servers := m.config.GetStringMap("api")
    for name := range servers {
        m.api.Add(name, server.WithEndpoint(...))
    }
    return nil
}
```

Per-server options read from config here: endpoint, TLS certs, and the
middleware configs mapping (`api.<name>.middlewares` →
`server.WithMiddlewareConfigs`) — see [api.md](../pkgs/api.md) and
[config.md](../pkgs/config.md).

## `api.go` — Middleware and Router Registration

```go
func (m *manager) initAPI() error {
    middlewares := []api.Middleware{
        authmw.New(),
    }
    routers := []api.Router{
        userRouter.New(m.userSvc, m.log),
    }
    if err := m.api.RegisterMiddlewares(middlewares...); err != nil {
        return errors.Wrap(err)
    }
    if err := m.api.RegisterRouters(routers...); err != nil {
        return errors.Wrap(err)
    }
    return nil
}
```

Middlewares before routers — router.yaml references them by name
([middlewares.md](middlewares.md)).

## `signal.go` — OS Signal Handling

The supervisor deliberately installs no signal handlers
([supervisor.md](../pkgs/supervisor.md)); this file is where they live:

```go
func (m *manager) listenSignals(ctx context.Context) {
    // SIGINT/SIGTERM  → graceful shutdown (services.Stop + cancel)
    // SIGUSR1         → dump service info to stdout (m.Info(os.Stdout, true))
    // SIGUSR2         → dump goroutine stack trace
}
```
