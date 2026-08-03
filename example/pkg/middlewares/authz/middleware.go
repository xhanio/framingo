package authz

import (
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	role role.Manager
}

func New(role role.Manager) fapi.Middleware {
	return &middleware{
		role: role,
	}
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
	return []common.Service{m.role}
}

// Func implements api.Middleware. The middleware takes no router.yaml config,
// so a block under its name is a mistake worth failing startup for.
func (m *middleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	if config != nil {
		return nil, errors.Newf("%s takes no config", m.Name())
	}
	return m.handle, nil
}

func (m *middleware) handle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		credential := c.Get(fapi.ContextKeyCredential)
		cred, ok := credential.(*entity.Credential)
		if cred == nil || !ok {
			return errors.Unauthorized
		}
		if cred.RequirePasswordReset {
			return errors.Forbidden.Newf("password reset required")
		}
		if !sliceutil.In(cred.Source, preset.AuthSourceLdapUser, preset.AuthSourceLocalUser) {
			return next(c)
		}
		// The Info middleware runs upstream and resolves the matched route's
		// metadata - its declared permission among it - onto the request context.
		req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo)
		if !ok || req == nil {
			return errors.Forbidden.Newf("no handler info for request %s %s", c.Request().Method, c.Request().URL.EscapedPath())
		}
		required := req.Permission
		if required == "" {
			return next(c) // public endpoint
		}
		allowed, err := m.role.HasPermission(c.Request().Context(), cred.Role, required)
		if err != nil {
			return errors.Forbidden.Wrap(err)
		}
		if !allowed {
			return errors.Forbidden.Newf("role %s lacks permission %s for %s %s", cred.Role, required, c.Request().Method, c.Request().URL.EscapedPath())
		}
		return next(c)
	}
}
