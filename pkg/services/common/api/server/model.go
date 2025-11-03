package server

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

type Server interface {
	common.Service
	common.Initializable
	common.Daemon
	Endpoint() *api.Endpoint
	RegisterRouters(routers ...api.Router) error
	RegisterMiddlewares(middlewares ...api.Middleware)
}
