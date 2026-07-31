// Package cors answers cross-origin requests with echo's permissive defaults:
// any origin, the simple methods, no credentials. That is a development
// setting — tighten the policy in New before exposing a deployment to
// browsers.
//
// CORS must see requests no route matches, since a preflight OPTIONS has no
// route of its own, so this middleware attaches server-wide through
// server.WithMiddlewares, not through router.yaml.
package cors

import (
	"path"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ fapi.Middleware = (*mw)(nil)

type mw struct {
	fn echo.MiddlewareFunc
}

func New() fapi.Middleware {
	return &mw{
		// For a restrictive policy, build the func from middleware.CORSConfig:
		// middleware.CORSWithConfig(middleware.CORSConfig{
		// 	AllowOrigins: []string{"https://app.example.com"},
		// 	AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		// 	AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		// })
		fn: middleware.CORS(),
	}
}

func (m *mw) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *mw) Dependencies() []common.Service {
	return nil
}

func (m *mw) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if config != nil {
		return nil, errors.Newf("%s takes no config: it attaches at server level, from code", m.Name())
	}
	return m.fn, nil
}
