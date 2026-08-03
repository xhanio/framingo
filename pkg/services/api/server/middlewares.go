package server

import (
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

// middlewares holds the server's lifecycle middlewares - recover, logger,
// info, error. They take no config and answer to no name, so they stay plain
// functions rather than paying for the api.Middleware contract; everything
// with a name and a config is the app's, attached through router.yaml or
// WithMiddlewares.
type middlewares struct {
	server *server
}

// newMiddleware creates a new middlewares instance for the given server
func newMiddleware(srv *server) *middlewares {
	return &middlewares{
		server: srv,
	}
}

// Error middlewares wraps and handles errors from handlers
func (mw *middlewares) Error(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err != nil {
			apiError := api.WrapError(err, c)
			apiError.Source = mw.server.Name()
			c.Set(api.ContextKeyError, apiError)
			return apiError
		}
		return nil
	}
}

// Info middlewares extracts request information and injects it into context
func (mw *middlewares) Info(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, found := mw.server.requestInfo(c)
		if !found {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		c.Set(api.ContextKeyRequestInfo, req)
		c.Set(api.ContextKeyTrace, req.TraceID)
		err := next(c)
		resp := mw.server.responseInfo(req.StartedAt, c)
		c.Set(api.ContextKeyResponseInfo, resp)
		return err
	}
}

// Logger middlewares logs request and response information
func (mw *middlewares) Logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().URL.EscapedPath())
		}
		resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
		if !ok || resp == nil {
			return errors.Newf("failed to get response from %s", c.Request().RequestURI)
		}
		if req.Poll {
			// TODO: stack polling api logs
		} else {
			mw.server.print(req, resp)
		}
		return err
	}
}

// Recover middlewares recovers from panics and converts them to errors
func (mw *middlewares) Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				mw.server.log.Error(string(debug.Stack()))
				var err error
				switch e := r.(type) {
				case errors.Error:
					err = errors.Wrapf(e, "!! recover from panic")
				case errors.Category:
					err = errors.Wrapf(e, "!! recover from panic")
				default:
					err = errors.Newf("!! recover from panic: %v", r)
				}
				c.Error(err)
			}
		}()
		return next(c)
	}
}
