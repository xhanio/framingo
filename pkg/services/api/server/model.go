package server

import (
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Server interface {
	common.Named
	Endpoint() *api.Endpoint
}

// Manager manages multiple server instances.
type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	Get(name string) (Server, error)
	List() []Server
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware) error
	Add(name string, opts ...ServerOption) error
}
