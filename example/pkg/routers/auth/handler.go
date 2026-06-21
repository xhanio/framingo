package auth

import (
	"net/http"
	"time"

	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/api"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (r *router) Login(c api.Context) error {
	var body api.LoginBody
	if err := c.Bind(&body); err != nil {
		return errors.BadRequest.Wrap(err)
	}
	if err := c.Validate(&body); err != nil {
		return errors.Wrap(err)
	}
	session, err := r.am.Login(c, preset.DefaultOrganizationName, body.Username, body.Password)
	if err != nil {
		return errors.Unauthorized.Wrap(err)
	}
	c.Set(fapi.ContextKeyCredential, session.Credential)
	cookie := &http.Cookie{
		Name:     fapi.CookiesKeySession,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		MaxAge:   int(preset.SessionCookieMaxAge.Seconds()),
	}
	c.SetCookie(cookie)
	c.Response().Header().Add(fapi.HeaderKeySession, session.ID)

	res := &api.LoginResponse{}
	if session.Credential != nil {
		res.RequirePasswordReset = session.Credential.RequirePasswordReset
	}
	return c.JSON(http.StatusOK, res)
}

func (r *router) Logout(c api.Context) error {
	if session, ok := c.Session(); ok && session != nil {
		r.am.CloseSession(c, session.ID)
	}
	cookie := &http.Cookie{
		Name:    fapi.CookiesKeySession,
		Expires: time.Now(),
	}
	c.SetCookie(cookie)
	return c.NoContent(http.StatusOK)
}

func (r *router) Session(c api.Context) error {
	credential, ok := c.Credential()
	if !ok || credential == nil {
		return errors.Unauthorized.New()
	}
	return c.JSON(http.StatusOK, credential)
}

func (r *router) Handlers() map[string]any {
	return api.DiscoverHandlers(r)
}
