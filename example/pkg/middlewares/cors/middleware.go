// Package cors must attach server-level, since a preflight OPTIONS matches no route;
// it stays dormant on a server whose middleware configs carry no "cors" entry.
package cors

import (
	"bytes"

	"github.com/labstack/echo/v4"
	em "github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"
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

func (m *middleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	if config == nil {
		return em.CORS(), nil
	}
	// strict: a typo in a security policy must fail startup, not keep the permissive defaults
	dec := yaml.NewDecoder(bytes.NewReader(config))
	dec.KnownFields(true)
	var cfg api.CORSConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, errors.Wrapf(err, "invalid cors config")
	}
	if cfg.AllowCredentials && len(cfg.AllowOrigins) == 0 {
		return nil, errors.Newf("invalid cors config: allow_credentials requires explicit allow_origins - browsers reject credentials against a wildcard origin")
	}
	ec := em.DefaultCORSConfig
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
	return em.CORSWithConfig(ec), nil
}
