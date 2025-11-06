package example

import (
	"compress/zlib"
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ api.Middleware = (*middleware)(nil)

type middleware struct {
}

func New() api.Middleware {
	return &middleware{}
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg) // use reflctutil to get package name as name
}

func (m *middleware) Dependencies() []common.Service {
	return nil
}

func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		if r.Header.Get("Content-Encoding") == "deflate" {
			reader, err := zlib.NewReader(r.Body)
			if err != nil {
				return errors.BadRequest.Newf("failed to deflate request body: %s", err)
			}
			// Set the new request body, which is the deflated data stream
			c.Request().Body = reader
		}
		return next(c)
	}
}
