package server

import (
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func (m *manager) apply(opts ...Option) {
	for _, opt := range opts {
		opt(m)
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

// ServerOption configures a server (echo server instance)
type ServerOption func(*server)

func (s *server) apply(opts ...ServerOption) {
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

func WithTLS(cert certutil.CertBundle, auth bool) ServerOption {
	return func(s *server) {
		s.tlsConfig = &api.ServerTLS{
			CertBundle:  cert,
			AuthEnabled: auth,
		}
	}
}

// WithMiddlewares installs middlewares on the server itself, ahead of the
// built-in chain and so ahead of any route match - where a concern that must
// answer requests no route matches, such as CORS preflight, has to sit. They
// run on every request to the server, are built with a nil config, and survive
// the echo rebuild a restart performs. RequestInfo does not exist yet at this
// position; a middleware that reads it belongs in router.yaml instead.
func WithMiddlewares(mws ...api.Middleware) ServerOption {
	return func(s *server) {
		s.middlewares = append(s.middlewares, mws...)
	}
}
