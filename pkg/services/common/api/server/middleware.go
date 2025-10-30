package server

import (
	"fmt"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
)

// Error middleware wraps and handles errors from handlers
func (s *server) Error(next echo.HandlerFunc) echo.HandlerFunc {
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
func (s *server) Info(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := s.requestInfo(c)
		if req == nil || req.Handler == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		c.Set(api.ContextKeyRequestInfo, req)
		c.Set(api.ContextKeyTrace, req.TraceID)
		err := next(c)
		resp := s.responseInfo(req.StartedAt, c)
		c.Set(api.ContextKeyResponseInfo, resp)
		return err
	}
}

// Logger middleware logs request and response information
func (s *server) Logger(next echo.HandlerFunc) echo.HandlerFunc {
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
			s.print(req, resp)
		}
		return err
	}
}

// Recover middleware recovers from panics and converts them to errors
func (s *server) Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error(string(debug.Stack()))
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
func (s *server) Throttle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, ok := c.Get(common.ContextKeyAPIRequestInfo).(*api.RequestInfo)
		if !ok || req == nil {
			return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
		}
		key := fmt.Sprintf("%s:%s", req.IP, req.Path)
		s.Lock()
		rl, ok := s.limits[key]
		if !ok {
			if req.Handler.ThrottleRPS != 0 {
				rl = rate.NewLimiter(req.Handler.ThrottleRPS, req.Handler.ThrottleBurstSize)
			} else if s.throttleConfig != nil {
				rl = rate.NewLimiter(s.throttleConfig.RPS, s.throttleConfig.BurstSize)
			} else {
				rl = nil
			}
			s.limits[key] = rl
		}
		if rl != nil && !rl.Allow() {
			s.Unlock()
			return errors.TooManyRequests.New(
				errors.WithMessage("you have been rate limited"),
				errors.WithCode("RATE_LIMIT", map[string]string{
					"ip": req.IP,
				}),
			)
		}
		s.Unlock()
		return next(c)
	}
}
