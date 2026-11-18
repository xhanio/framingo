package server

import (
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Server interface {
	common.Named
	Endpoint() *api.Endpoint
}

type Manager interface {
	common.Service
	common.Liveness
	common.Readiness
	common.Initializable
	common.Daemon
	Get(name string) (Server, error)
	List() []Server
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware) error
	// Add returns a Conflict for a name already added.
	Add(name string, opts ...ServerOption) error
}
