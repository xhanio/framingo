package server

import (
	"crypto/x509"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

type Option func(*manager)

func (m *manager) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *manager) setDefaults() {
	if m.log == nil {
		m.log = log.Default
	}
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func WithDebug(enabled bool) Option {
	return func(m *manager) {
		m.debug = enabled
	}
}

type ServerOption func(*server)

func (s *server) applyOptions(opts ...ServerOption) {
	for _, opt := range opts {
		opt(s)
	}
}

func WithEndpoint(host string, port uint, prefix string) ServerOption {
	return func(s *server) {
		if host != "" && port > 0 {
			s.endpoint = &api.Endpoint{
				Host: host,
				Port: port,
				Path: prefix,
			}
		}
	}
}

// WithTLS serves https with cert, verifying a client certificate, when one is
// given, against clientCAs; none is asked for without them.
func WithTLS(cert certutil.CertBundle, clientCAs ...*x509.Certificate) ServerOption {
	return func(s *server) {
		s.tlsConfig = &api.ServerTLS{
			CertBundle: cert,
			ClientCAs:  clientCAs,
		}
	}
}

// WithMeter enables HTTP metrics; the meter is not a dependency, so a collector outage never takes down the listener.
func WithMeter(meter model.Meter) ServerOption {
	return func(s *server) {
		s.meter = meter
	}
}

// WithMiddlewares runs on every request, matched or not (e.g. cors), before RequestInfo exists.
func WithMiddlewares(mws ...api.Middleware) ServerOption {
	return func(s *server) {
		s.middlewares = append(s.middlewares, mws...)
	}
}

// WithMiddlewareConfigs maps name to a switch (boolean) or config block; a WithMiddlewares entry without a key stays dormant.
func WithMiddlewareConfigs(defaults []byte) ServerOption {
	return func(s *server) {
		s.middlewareConfigs = defaults
	}
}
