// Package throttle rate-limits requests per client IP. Attach it through
// router.yaml — at group level to cover a router, or per handler — and give a
// route its own limit right there:
//
//	middlewares:
//	  - throttle            # the server's default limit for this middleware
//	  - throttle: false     # or switched off for this route
//	  - throttle:           # or this route's own limit, overriding it
//	      rps: 1
//	      burst_size: 3
//
// The bare form takes its config from the group's entry, then from the
// server's middleware configs (server.WithMiddlewareConfigs), which is where a
// server-wide limit belongs.
// Each attachment is one route and keeps its own limiter table, so the key is
// the client IP alone. An attachment that ends up with no limit at all passes
// everything, which lets a router attach the middleware unconditionally and
// leave the limit to configuration.
package throttle

import (
	"path"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct{}

func New() fapi.Middleware {
	return &middleware{}
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
	return nil
}

// Func builds the attachment for one route: its limit, and its own limiter
// table, live in the returned closure.
func (m *middleware) Func(enabled bool, raw []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	var cfg api.ThrottleConfig
	if raw != nil {
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, errors.Wrapf(err, "invalid throttle config")
		}
	}
	rps, burst := cfg.RPS, cfg.BurstSize
	if rps == 0 || burst == 0 {
		// No limit for this route: pass everything without bookkeeping.
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
	}

	var mu sync.RWMutex
	limits := make(map[string]*rate.Limiter)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// The Info middleware runs upstream and resolves the client IP
			// onto the request context.
			req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo)
			if !ok || req == nil {
				return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
			}

			// Fast path: check if limiter exists (read lock)
			mu.RLock()
			rl, ok := limits[req.IP]
			mu.RUnlock()

			// Slow path: create limiter if it doesn't exist (write lock)
			if !ok {
				mu.Lock()
				// Double-check after acquiring write lock
				rl, ok = limits[req.IP]
				if !ok {
					rl = rate.NewLimiter(rate.Limit(rps), burst)
					limits[req.IP] = rl
				}
				mu.Unlock()
			}

			if !rl.Allow() {
				return errors.TooManyRequests.New(
					errors.WithMessage("you have been rate limited"),
					errors.WithCode("RATE_LIMIT", map[string]string{
						"ip": req.IP,
					}),
				)
			}
			return next(c)
		}
	}, nil
}
