# Framingo Config YAML Reference

Reference config structure for all framework services. Use this as a template when creating new applications.

## Example Config YAML

```yaml
# Logging — used by log.New() in service.go
log:
  level: 0                    # 0=Debug, 1=Info, 2=Warn, 3=Error
  file: /var/log/myapp/app.log
  rotation:
    max_size: 100             # MB per log file
    max_backups: 3            # number of rotated files to keep
    max_age: 7                # days to retain old log files

# Database — used by db.New() options + dynamic config in db.Manager.Init()
db:
  type: postgres              # postgres | mysql | sqlite | clickhouse
  source:
    host: localhost
    port: 5432
    user: myapp
    password: secret
    dbname: myapp_db
  migration:
    dir: ./migrations         # path to migration SQL files
    version: 0                # target version (0 = latest)
  connection:
    max_open: 10              # max open connections
    max_idle: 5               # max idle connections
    max_lifetime: 1h          # connection max lifetime
    max_idle_time: 30m        # idle connection max lifetime
    exec_timeout: 30s         # query execution timeout

# API servers — iterated by m.config.GetStringMap("api") in service.go
# Each key becomes a named server instance via m.api.Add(name, ...)
api:
  http:                       # server name: "http"
    host: 0.0.0.0
    port: 8080
    prefix: /api/v1           # server endpoint path
    throttle:                 # optional: server-wide rate limiting
      rps: 100.0
      burst_size: 200
  admin:                      # server name: "admin"
    host: 0.0.0.0
    port: 9090
    prefix: /admin
  # HTTPS example with TLS
  # https:
  #   host: 0.0.0.0
  #   port: 443
  #   prefix: /api/v1
  #   cert: /path/to/server.crt
  #   key: /path/to/server.key

# TLS CA certificate (used when any api server has cert/key configured)
# ca:
#   cert: /path/to/ca.crt

# pprof profiling — optional, set port to enable
pprof:
  port: 6060                  # 0 = disabled

# Custom service config — read in service Init() via confutil.FromContext(ctx)
# example:
#   greeting: hello world!
```

**Notes**:
- `db.connection.*` keys are read dynamically during `db.Manager.Init(ctx)` via `confutil.FromContext(ctx)`, allowing values to change on service restart
- `api.*` is iterated as a string map — each top-level key under `api` becomes a named server instance
- TLS is enabled per-server when `api.<name>.cert` is set
- Throttle is enabled per-server when `api.<name>.throttle` is set
- Custom service config keys are accessed in `Init(ctx)` via `confutil.FromContext(ctx).GetString("myservice.key")`
- Pubsub and planner services are configured entirely via functional options, not YAML keys
