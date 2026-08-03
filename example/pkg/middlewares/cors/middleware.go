// Package cors answers cross-origin requests, the one concern that must run
// ahead of routing: a preflight OPTIONS matches no route, so no
// route-attached middleware could ever see it. Attach it server-level -
// server.WithMiddlewares(cors.New()) at Add time - and activate it per
// server through the middleware configs under "cors":
//
//	cors: true      # or `cors:` - echo's permissive defaults, a development setting
//	cors:           # a policy tightens it field by field
//	  allow_origins:
//	    - https://app.example.com
//
// With no cors entry at all the attachment stays dormant, so a server whose
// config carries none - the health listener, say - serves without it.
package cors

import (
	"bytes"
	"path"

	"github.com/labstack/echo/v4"
	em "github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
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
	return path.Base(pkg) // package name == middleware name
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
