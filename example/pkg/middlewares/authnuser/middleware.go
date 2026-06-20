package authnuser

import (
	"net/http"
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/services/system/auth"
	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	log  log.Logger
	auth auth.Manager
	role role.Manager
}

func New(auth auth.Manager, role role.Manager, opts ...Option) fapi.Middleware {
	m := &middleware{
		auth: auth,
		role: role,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
	return []common.Service{m.auth, m.role}
}

func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Request().Header.Get(fapi.HeaderKeyAPIToken)
		if token != "" {
			credential, err := m.auth.AuthenticateAPIToken(c.Request().Context(), token)
			if err != nil {
				return errors.Unauthorized.Wrap(err)
			}
			permissions, err := m.role.GetPermissionsByName(c.Request().Context(), credential.Role)
			if err != nil {
				return errors.Internal.Wrap(err)
			}
			credential.Permissions = permissions
			c.Set(fapi.ContextKeyCredential, credential)
		} else {
			sessionID := sliceutil.First(
				c.Request().Header.Get(fapi.HeaderKeySession),
				c.QueryParam(fapi.QueryParamSession),
			)
			if sessionID == "" {
				cookie, err := c.Cookie(fapi.CookiesKeySession)
				if err != nil {
					switch err {
					case http.ErrNoCookie:
						return errors.Unauthorized.Newf("no session header or cookie available")
					default:
						return errors.BadRequest.Wrap(err)
					}
				}
				if cookie.Value == "" {
					return errors.Unauthorized.Newf("session cookie value is empty")
				}
				sessionID = cookie.Value
			}
			session, ok := m.auth.GetSession(c.Request().Context(), sessionID)
			if !ok {
				return errors.Unauthorized.Newf("session expired already, please login again")
			}
			// only action will lead to session refresh if there are polling in the future
			// if c.Request().Method != http.MethodGet {
			session.Lease.Refresh(preset.SessionExpiration)
			// }
			permissions, err := m.role.GetPermissionsByName(c.Request().Context(), session.Credential.Role)
			if err != nil {
				return errors.Internal.Wrap(err)
			}
			session.Credential.Permissions = permissions
			c.Set(fapi.ContextKeySession, session)
			c.Set(fapi.ContextKeyCredential, session.Credential)
		}
		return next(c)
	}
}
