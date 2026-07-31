// Package throttle rate-limits requests per client IP. Attach it through
// router.yaml — at group level to cover a router, or per handler — and give a
// route its own limit right there:
//
//	middlewares:
//	  - throttle            # the instance limit from WithLimit
//	  - throttle:           # or this route's own, overriding it
//	      rps: 1
//	      burst_size: 3
//
// Each attachment is one route and keeps its own limiter table, so the key is
// the client IP alone. An attachment that ends up with no limit — no config,
// no instance limit — passes everything, which lets a router attach the
// middleware unconditionally and leave the limit to configuration.
package throttle

import (
	"path"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	rps   float64
	burst int
}

// config is the router.yaml block under this middleware's name. When present
// it replaces the instance limit for that route entirely, zeros meaning
// unthrottled.
type config struct {
	RPS       float64 `yaml:"rps"`
	BurstSize int     `yaml:"burst_size"`
}

func New(opts ...Option) fapi.Middleware {
	m := &middleware{}
	for _, opt := range opts {
		opt(m)
	}
	return m
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
func (m *middleware) Func(raw []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	rps, burst := m.rps, m.burst
	if raw != nil {
		var cfg config
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, errors.Wrapf(err, "invalid throttle config")
		}
		rps, burst = cfg.RPS, cfg.BurstSize
	}
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
