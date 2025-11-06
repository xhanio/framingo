package server

import (
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

// Server represents a single echo server instance
type Server interface {
	common.Named
	Endpoint() *api.Endpoint
	Routers() []*api.HandlerGroup
	HandlerPath(group *api.HandlerGroup, handler *api.Handler) string
}

// Manager manages multiple server instances
type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	Add(name string, opts ...ServerOption) error
	Get(name string) (Server, error)
	List() []Server
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware) error
}
