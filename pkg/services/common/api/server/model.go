package server

import (
	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

type Server interface {
	common.Service
	common.Initializable
	common.Daemon
	RegisterRouters(routers []*api.Router, middlewares ...echo.MiddlewareFunc) error
	RegisterMiddlewares(middlewares ...echo.MiddlewareFunc)
	ServerPrefix(router *api.Router) string
}
