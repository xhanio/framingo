package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Application interface {
	Register(services ...common.Service)
	TopoSort() error
	Services() []common.Service
	Stats() ([]*entity.ApplicationStats, error)
	// Migrate() error
	InitService(ctx context.Context, name string) error
	StartService(name string) error
	StopService(name string, wait bool) error
	RestartService(ctx context.Context, name string) error
}
