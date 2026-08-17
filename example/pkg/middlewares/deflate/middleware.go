package deflate

import (
	"compress/zlib"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/ioutil"
)

var _ api.Middleware = (*middleware)(nil)

// DefaultMaxDecompressed caps how many bytes a single request body may expand
// to. Any body-size limit upstream applies to the COMPRESSED bytes, so without
// a cap here a few hundred KB of zlib can expand to gigabytes and exhaust
// memory — a classic decompression bomb.
const DefaultMaxDecompressed = 32 << 20 // 32 MiB

type middleware struct {
	name            string
	maxDecompressed int
}

func New(opts ...Option) api.Middleware {
	m := &middleware{}
	m.applyOptions(opts...)
	m.setDefaults()
	return m
}

func (m *middleware) Name() string {
	return m.name
}

func (m *middleware) Dependencies() []common.Service {
	return nil
}

// Func implements api.Middleware. The middleware takes no router.yaml config,
// so a block under its name is a mistake worth failing startup for.
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
		r := c.Request()
		if r.Header.Get("Content-Encoding") == "deflate" {
			reader, err := zlib.NewReader(r.Body)
			if err != nil {
				return errors.BadRequest.Newf("failed to deflate request body: %s", err)
			}
			// Set the new request body, which is the deflated data stream,
			// bounded so a small payload cannot expand without limit.
			// NewLimitReader is a ReadCloser, so it drops straight in; its
			// Close releases the zlib reader. net/http closes the original
			// body itself, using the reference it captured before handlers ran.
			c.Request().Body = ioutil.NewLimitReader(reader, m.maxDecompressed)
		}
		return next(c)
	}
}
