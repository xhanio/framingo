package api

import (
	"time"

	"github.com/labstack/echo/v4"
)

var _ Context = (*DefaultContext)(nil)

type DefaultContext struct {
	echo.Context
}

func (c *DefaultContext) Deadline() (time.Time, bool) {
	return c.Request().Context().Deadline()
}

func (c *DefaultContext) Done() <-chan struct{} {
	return c.Request().Context().Done()
}

func (c *DefaultContext) Err() error {
	return c.Request().Context().Err()
}

func (c *DefaultContext) Value(key any) any {
	if k, ok := key.(string); ok {
		return c.Get(k)
	}
	return nil
}

func (c *DefaultContext) BindQuery() *echo.ValueBinder {
	return echo.QueryParamsBinder(c.Context)
}

func (c *DefaultContext) BindPath() *echo.ValueBinder {
	return echo.PathParamsBinder(c.Context)
}

func (c *DefaultContext) BindForm() *echo.ValueBinder {
	return echo.FormFieldBinder(c.Context)
}

func (c *DefaultContext) BindAny(i any) error {
	return c.Context.Bind(i)
}

func DefaultHandlerWrapper(hf HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return hf(&DefaultContext{
			Context: c,
		})
	}
}
