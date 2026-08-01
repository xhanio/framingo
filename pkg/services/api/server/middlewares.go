package server

import (
	"bytes"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

// middlewares holds the server's lifecycle middlewares - recover, logger,
// info, error. They take no config and answer to no name, so they stay plain
// functions rather than paying for the api.Middleware contract; cors, the one
// built-in that is configured, implements it below like any user middleware.
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

var _ api.Middleware = (*corsMiddleware)(nil)

// corsMiddleware answers cross-origin requests, the one concern that must run
// ahead of routing: a preflight OPTIONS matches no route, so no route-attached
// middleware could ever see it. It is browser protocol rather than app policy,
// which is why the server ships it - as a standard api.Middleware, configured
// through the server's middleware configs under "cors" like any other, so that
// name is the server's own. Unconfigured or false it declines attachment; true
// enables echo's permissive defaults - a development setting; an api.CORSConfig
// mapping tightens the policy field by field.
type corsMiddleware struct{}

func (m *corsMiddleware) Name() string {
	return "cors"
}

func (m *corsMiddleware) Dependencies() []common.Service {
	return nil
}

func (m *corsMiddleware) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if config == nil {
		return nil, nil
	}
	var enabled bool
	if err := yaml.Unmarshal(config, &enabled); err == nil {
		if !enabled {
			return nil, nil
		}
		return middleware.CORS(), nil
	}
	// Decode strictly: a typo'd field in a security policy must fail startup,
	// not silently leave the permissive defaults in place.
	dec := yaml.NewDecoder(bytes.NewReader(config))
	dec.KnownFields(true)
	var cfg api.CORSConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, errors.Wrapf(err, "invalid cors config")
	}
	if cfg.AllowCredentials && len(cfg.AllowOrigins) == 0 {
		return nil, errors.Newf("invalid cors config: allow_credentials requires explicit allow_origins - browsers reject credentials against a wildcard origin")
	}
	ec := middleware.DefaultCORSConfig
	if len(cfg.AllowOrigins) > 0 {
		ec.AllowOrigins = cfg.AllowOrigins
	}
	if len(cfg.AllowMethods) > 0 {
		ec.AllowMethods = cfg.AllowMethods
	}
	if len(cfg.AllowHeaders) > 0 {
		ec.AllowHeaders = cfg.AllowHeaders
	}
	ec.AllowCredentials = cfg.AllowCredentials
	ec.MaxAge = cfg.MaxAge
	return middleware.CORSWithConfig(ec), nil
}
