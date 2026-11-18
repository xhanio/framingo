// Package throttle rate-limits per client IP. Each attachment (route) keeps its own limiter table;
// a bare attachment takes its limit from the group's entry, then the server's middleware configs,
// and one with no limit passes everything.
package throttle

import (
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	name string
}

func New(opts ...Option) fapi.Middleware {
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
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
	}

	var mu sync.RWMutex
	limits := make(map[string]*rate.Limiter)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// set upstream by the server's Info middleware
			req, ok := c.Get(fapi.ContextKeyRequestInfo).(*fapi.RequestInfo)
			if !ok || req == nil {
				return errors.NotFound.Newf("failed to look up handler %s", c.Request().RequestURI)
			}

			mu.RLock()
			rl, ok := limits[req.IP]
			mu.RUnlock()

			if !ok {
				mu.Lock()
				// re-check under the write lock
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
