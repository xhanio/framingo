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

func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		credential := c.Get(fapi.ContextKeyCredential)
		cred, ok := credential.(*entity.Credential)
		if cred == nil || !ok {
			return errors.Unauthorized
		}
		if cred.RequirePasswordReset {
			return errors.Forbidden.Newf("password reset required")
		}

		if sliceutil.In(cred.Source, preset.AuthSourceLdapUser, preset.AuthSourceLocalUser) {
			if ok, err := m.role.CheckPermissionByName(c.Request().Context(), cred.Role, c.Request().Method, c.Request().URL.EscapedPath()); err != nil {
				return errors.Forbidden.Wrap(err)
			} else if !ok {
				return errors.Forbidden.Newf("no permission for role %s on request %s %s", c.Request().Method, c.Request().URL.EscapedPath())
			}
		}
		return next(c)
	}
}
