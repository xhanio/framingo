package server

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

// middleware holds the middleware functions for a specific server
type middleware struct {
	server *server

	sync.RWMutex // lock for rate limiters
	limits       map[string]*rate.Limiter
}

// newMiddleware creates a new middleware instance for the given server
func newMiddleware(srv *server) *middleware {
	return &middleware{
		server: srv,
		limits: make(map[string]*rate.Limiter),
	}
}

// Error middleware wraps and handles errors from handlers
func (mw *middleware) Error(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err != nil {
			apiError := api.WrapError(err, c)
			c.Set(api.ContextKeyError, apiError)
			return apiError
		}
		return nil
	}
}

// Info middleware extracts request information and injects it into context
func (mw *middleware) Info(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := mw.server.requestInfo(c)
		if req == nil || req.Handler == nil {
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

// Logger middleware logs request and response information
func (mw *middleware) Logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil || req.Handler == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().URL.EscapedPath())
		}
		resp, ok := c.Get(common.ContextKeyAPIResponseInfo).(*api.ResponseInfo)
		if !ok || resp == nil {
			return errors.Newf("failed to get response from %s", c.Request().RequestURI)
		}
		if req.Handler.Poll {
			// TODO: stack polling api logs
		} else {
			mw.server.print(req, resp)
		}
		return err
	}
}

// Recover middleware recovers from panics and converts them to errors
func (mw *middleware) Recover(next echo.HandlerFunc) echo.HandlerFunc {
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

// Throttle middleware implements rate limiting per IP and path
func (mw *middleware) Throttle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}

		// Use the server's throttle config directly
		serverThrottleConfig := mw.server.throttleConfig

		key := fmt.Sprintf("%s:%s", req.IP, req.Path)

		// Fast path: check if limiter exists (read lock)
		mw.RLock()
		rl, ok := mw.limits[key]
		mw.RUnlock()

		// Slow path: create limiter if it doesn't exist (write lock)
		if !ok {
			mw.Lock()
			// Double-check after acquiring write lock
			rl, ok = mw.limits[key]
			if !ok {
				if req.Handler.Throttle != nil {
					rl = rate.NewLimiter(req.Handler.Throttle.RPS, req.Handler.Throttle.BurstSize)
				} else if serverThrottleConfig != nil {
					rl = rate.NewLimiter(serverThrottleConfig.RPS, serverThrottleConfig.BurstSize)
				} else {
					rl = nil
				}
				mw.limits[key] = rl
			}
			mw.Unlock()
		}

		if rl != nil && !rl.Allow() {
			return errors.TooManyRequests.New(
				errors.WithMessage("you have been rate limited"),
				errors.WithCode("RATE_LIMIT", map[string]string{
					"ip": req.IP,
				}),
			)
		}
		return next(c)
	}
}
