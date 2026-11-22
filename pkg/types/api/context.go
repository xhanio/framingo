package api

import (
	"context"

	"github.com/labstack/echo/v4"
)

// SetContext replaces c's request with one carrying ctx: how a middleware hands a value
// to everything below it, since echo keeps no context of its own and the request's is
// the one (design/request-context.md).
func SetContext(c echo.Context, ctx context.Context) {
	c.SetRequest(c.Request().WithContext(ctx))
}
