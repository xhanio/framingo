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
	// health.go: a listener that stopped serving fails both probes - a
	// restart rebuilds the echo instances and re-binds it, so it is
	// liveness-fixable, and it is equally not ready for traffic.
	common.Liveness
	common.Readiness
	common.Initializable
	common.Daemon
	Get(name string) (Server, error)
	List() []Server
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware) error
	Add(name string, opts ...ServerOption) error
}
