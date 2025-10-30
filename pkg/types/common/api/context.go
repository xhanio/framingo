package api

import (
	"context"

	"github.com/labstack/echo/v4"
)

type Context interface {
	echo.Context
	context.Context
	BindQuery() *echo.ValueBinder
	BindPath() *echo.ValueBinder
	BindForm() *echo.ValueBinder
	BindAny(i any) error
}
