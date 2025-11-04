package server

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

// Server represents a single echo server instance
type Server interface {
	common.Named
	Routers() (map[string]*api.HandlerGroup, map[string]*api.Handler)
}

// Manager manages multiple server instances
type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	AddServer(name string, opts ...ServerOption) error
	GetServer(name string) Server
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware)
}
