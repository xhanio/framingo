package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Supervisor interface {
	common.Service
	// Register adds services and the dependencies they declare; it is for
	// wiring, before Init. A name stands for one instance: meeting the same
	// instance again is a no-op, but another instance under a name already
	// taken is refused with a Conflict - the dependency graph, the stats and
	// the by-name operations all key on the name, so the second instance
	// would otherwise never run. The error is a wiring mistake to fail
	// startup on.
	Register(services ...common.Service) error
	TopoSort() error
	Services() []common.Service
	Stats() ([]*entity.SupervisorStats, error)
	// Migrate() error
	InitService(ctx context.Context, name string) error
	StartService(name string) error
	StopService(ctx context.Context, name string) error
	RestartService(ctx context.Context, name string) error
	Restart(ctx context.Context) error
}
