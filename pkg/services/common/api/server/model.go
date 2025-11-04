package server

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	AddServer(name string, opts ...ServerOption)
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware)
	Routers() (map[string]*api.HandlerGroup, map[string]*api.Handler)
}
