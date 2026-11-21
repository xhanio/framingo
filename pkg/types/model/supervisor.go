package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Supervisor interface {
	common.Service
	// Register, before Init, also adds declared dependencies; re-registering an
	// instance is a no-op, a second instance under a taken name is a Conflict.
	Register(services ...common.Service) error
	TopoSort() error
	Services() []common.Service
	Stats() ([]*entity.SupervisorStats, error)
	// Migrate() error
	InitService(ctx context.Context, name string) error
	StartService(ctx context.Context, name string) error
	StopService(ctx context.Context, name string) error
	RestartService(ctx context.Context, name string) error
	Restart(ctx context.Context) error
}
