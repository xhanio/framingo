package server

import (
	"fmt"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

// Error middleware wraps and handles errors from handlers
func (m *manager) Error(next echo.HandlerFunc) echo.HandlerFunc {
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
func (m *manager) Info(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := m.requestInfo(c)
		if req == nil || req.Handler == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		c.Set(api.ContextKeyRequestInfo, req)
		c.Set(api.ContextKeyTrace, req.TraceID)
		err := next(c)
		resp := m.responseInfo(req.StartedAt, c)
		c.Set(api.ContextKeyResponseInfo, resp)
		return err
	}
}

// Logger middleware logs request and response information
func (m *manager) Logger(next echo.HandlerFunc) echo.HandlerFunc {
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
			m.print(req, resp)
		}
		return err
	}
}

// Recover middleware recovers from panics and converts them to errors
func (m *manager) Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				m.log.Error(string(debug.Stack()))
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
func (m *manager) Throttle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}

		// Get the server's throttle config from the handler group's server
		var serverThrottleConfig *api.ThrottleConfig
		if req.HandlerGroup != nil && req.HandlerGroup.Server != "" {
			if srv, ok := m.servers[req.HandlerGroup.Server]; ok {
				serverThrottleConfig = srv.throttleConfig
			}
		}

		key := fmt.Sprintf("%s:%s", req.IP, req.Path)
		m.Lock()
		rl, ok := m.limits[key]
		if !ok {
			if req.Handler.Throttle != nil {
				rl = rate.NewLimiter(req.Handler.Throttle.RPS, req.Handler.Throttle.BurstSize)
			} else if serverThrottleConfig != nil {
				rl = rate.NewLimiter(serverThrottleConfig.RPS, serverThrottleConfig.BurstSize)
			} else {
				rl = nil
			}
			m.limits[key] = rl
		}
		if rl != nil && !rl.Allow() {
			m.Unlock()
			return errors.TooManyRequests.New(
				errors.WithMessage("you have been rate limited"),
				errors.WithCode("RATE_LIMIT", map[string]string{
					"ip": req.IP,
				}),
			)
		}
		m.Unlock()
		return next(c)
	}
}
