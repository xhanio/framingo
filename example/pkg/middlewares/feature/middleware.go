package feature

import (
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
}

func New() fapi.Middleware {
	return &middleware{}
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
	return nil
}

func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*fapi.RequestInfo)
		if !ok || req == nil || req.Handler == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		if req.Handler.Permission == "" {
			return next(c)
		}
		features := rbac.Features[rbac.FeatureBasic]
		features = sliceutil.Deduplicate(features...)
		if sliceutil.In(req.Handler.Permission, features...) {
			return next(c)
		}
		return errors.Forbidden.Newf("Access denied to this feature. Please upload an appropriate license to enable this functionality.")
	}
}
