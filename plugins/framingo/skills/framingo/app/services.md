# Services — Writing Your Own

Every business capability is a service package under `pkg/services/<name>/`.
The lifecycle interfaces it composes and the supervisor that runs it are
framework reference — [supervisor.md](../pkgs/supervisor.md); this file is
the authoring convention.

Always an **unexported struct** with an **exported interface** and factory function — a strict convention throughout framingo.

## The Interface Goes in Two Places

This trips people up because both halves get called "the service interface". Templates: [`_templates/types-model-order.go`](../_templates/types-model-order.go) and [`_templates/services-order-model.go`](../_templates/services-order-model.go).

| File | Declares | Who depends on it |
|---|---|---|
| `pkg/types/model/order.go` | `model.Order` — business methods + `common.Service`, **no lifecycle** | Routers and other services |
| `pkg/services/order/model.go` | `order.Manager` = `model.Order` + the lifecycle interfaces it implements | Only `pkg/components/server/` wiring |

A router takes `model.Order`, never `order.Manager`. That's what keeps the implementation package-private and stops services importing each other. If a router imports `pkg/services/...`, the split has been skipped.

## The Package Skeleton

```go
package myservice

import (
    "context"
    "path"

    "github.com/xhanio/framingo/pkg/services/db"
    "github.com/xhanio/framingo/pkg/types/common"
    "github.com/xhanio/framingo/pkg/utils/log"
    "github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// Exported interface — the public API contract
type Manager interface {
    common.Service
    common.Initializable
    DoSomething(ctx context.Context) error
}

// Unexported struct — implementation detail
type manager struct {
    name string
    log  log.Logger
    db   db.Manager
}

// Factory function returns the exported interface; newManager returns the
// concrete type, so package tests construct *manager without the interface
// in the way.
func New(database db.Manager, opts ...Option) Manager {
    return newManager(database, opts...)
}

func newManager(database db.Manager, opts ...Option) *manager {
    m := &manager{
        log: log.Default,
        db:  database,
    }
    m.apply(opts...)
    m.log = m.log.By(m)
    return m
}

func (m *manager) Name() string {
    if m.name == "" {
        m.name = path.Join(reflectutil.Locate(m))
    }
    return m.name
}

func (m *manager) Dependencies() []common.Service {
    return []common.Service{m.db}
}

func (m *manager) Init(ctx context.Context) error {
    // Called on startup and restart. Read config, set up resources.
    return nil
}
```
