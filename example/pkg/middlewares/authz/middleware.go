package authz

import (
	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	name string
	role role.Manager
}

func New(role role.Manager, opts ...Option) fapi.Middleware {
	m := &middleware{
		role: role,
	}
	m.applyOptions(opts...)
	m.setDefaults()
	return m
}

func (m *middleware) Name() string {
	return m.name
}

func (m *middleware) Dependencies() []common.Service {
	return []common.Service{m.role}
}

// Func takes no router.yaml config: a block under its name fails startup.
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
		// set upstream by the server's Info middleware
		req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo)
		if !ok || req == nil {
			return errors.Forbidden.Newf("no handler info for request %s %s", c.Request().Method, c.Request().URL.EscapedPath())
		}
		// fail closed: a route naming no permission is a forgotten declaration; open routes set `- authz: false`
		required := req.Permission
		if required == "" {
			return errors.Forbidden.Newf("%s %s declares no permission", c.Request().Method, c.Request().URL.EscapedPath())
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
