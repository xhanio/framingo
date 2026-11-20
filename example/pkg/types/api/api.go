// Package api wraps echo.Context in a Context that is also a context.Context,
// and adapts handlers taking it to the framework's echo signatures.
package api

import (
	"context"
	"reflect"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Context interface {
	echo.Context
	context.Context
	BindQuery() *echo.ValueBinder
	BindPath() *echo.ValueBinder
	BindForm() *echo.ValueBinder
	BindAny(i any) error

	Credential() (*entity.Credential, bool)
	Session() (*entity.Session, bool)
	TraceID() (string, bool)
}

// ctx takes Deadline/Done/Err from the request's context but answers Value from echo's store.
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
	return c.Request().Context().Value(key)
}

func (c *ctx) Credential() (*entity.Credential, bool) {
	return entity.CredentialFrom(c)
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
	return c.Bind(i)
}

type WebSocketHandlerFunc func(Context, *websocket.Conn) error

func WrapWebSocket(wf WebSocketHandlerFunc) func(echo.Context, *websocket.Conn) error {
	return func(c echo.Context, conn *websocket.Conn) error {
		return wf(&ctx{c}, conn)
	}
}

type HandlerFunc func(Context) error

func WrapHandler(hf HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return hf(&ctx{c})
	}
}

// DiscoverHandlers wraps r's methods with a known handler signature, keyed by name; Handlers itself is skipped.
func DiscoverHandlers(r any) map[string]any {
	handlers := make(map[string]any)
	rv := reflect.ValueOf(r)
	rt := reflect.TypeOf(r)
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		if method.Name == "Handlers" {
			continue
		}
		switch fn := rv.Method(i).Interface().(type) {
		case func(echo.Context) error:
			handlers[method.Name] = echo.HandlerFunc(fn)
		case func(Context) error:
			handlers[method.Name] = WrapHandler(fn)
		case func(echo.Context, *websocket.Conn) error:
			handlers[method.Name] = fn
		case func(Context, *websocket.Conn) error:
			handlers[method.Name] = WrapWebSocket(fn)
		}
	}
	return handlers
}
