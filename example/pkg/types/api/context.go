package api

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Context interface {
	echo.Context
	context.Context
	Credential() (*entity.Credential, bool)
	Session() (*entity.Session, bool)
	TraceID() (string, bool)
	BindQuery() *echo.ValueBinder
	BindPath() *echo.ValueBinder
	BindForm() *echo.ValueBinder
	BindAny(i any) error
}

type ctx struct {
	echo.Context
}

func (c *ctx) Deadline() (time.Time, bool) {
	return c.Request().Context().Deadline()
}

func (c *ctx) Done() <-chan struct{} {
	return c.Request().Context().Done()
}

func (c *ctx) Err() error {
	return c.Request().Context().Err()
}

func (c *ctx) Value(key any) any {
	if k, ok := key.(string); ok {
		return c.Get(k)
	}
	return nil
}

func (c *ctx) Credential() (*entity.Credential, bool) {
	credential := c.Get(api.ContextKeyCredential)
	if credential == nil {
		return nil, false
	}
	cred, ok := credential.(*entity.Credential)
	if !ok {
		return nil, false
	}
	return cred, true
}

func (c *ctx) Session() (*entity.Session, bool) {
	session := c.Get(api.ContextKeySession)
	if session == nil {
		return nil, false
	}
	sess, ok := session.(*entity.Session)
	if !ok {
		return nil, false
	}
	return sess, true
}

func (c *ctx) TraceID() (string, bool) {
	traceID := c.Get(api.ContextKeyTrace)
	if traceID == nil {
		return "", false
	}
	tid, ok := traceID.(string)
	return tid, ok
}

func (c *ctx) BindQuery() *echo.ValueBinder {
	return echo.QueryParamsBinder(c.Context)
}

func (c *ctx) BindPath() *echo.ValueBinder {
	return echo.PathParamsBinder(c.Context)
}

func (c *ctx) BindForm() *echo.ValueBinder {
	return echo.FormFieldBinder(c.Context)
}

func (c *ctx) BindAny(i any) error {
	return c.Context.Bind(i)
}

type HandlerFunc func(Context) error

func WrapHandler(hf HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return hf(&ctx{c})
	}
}
