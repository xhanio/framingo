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

// WithMiddlewares installs middlewares on the server itself, running on every
// request - matched or not - inside the built-in recover and cors but ahead of
// the rest of the lifecycle. Each is built with the server's middleware config
// under its name and survives the echo rebuild a restart performs. RequestInfo
// does not exist yet at this position; a middleware that reads it belongs in
// router.yaml instead.
func WithMiddlewares(mws ...api.Middleware) ServerOption {
	return func(s *server) {
		s.middlewares = append(s.middlewares, mws...)
	}
}

// WithMiddlewareConfigs sets the server's default middleware configs, a plain
// mapping of middleware name to config - unlike a router.yaml middleware list,
// which stays a sequence because attachment order matters there:
//
//	cors: true
//	throttle:
//	  rps: 100.0
//	  burst_size: 200
//
// The server's built-in cors middleware reads its block from the same mapping
// under "cors", so that name is the server's own; the lifecycle built-ins
// (recover, logger, info, error) take no config and claim no names.
//
// Each block is what a middleware's Func receives when nothing more specific
// exists. Config resolves most-specific-first: a handler entry's own block,
// else its group entry's, else this server config, else nil - and server-level
// middlewares (WithMiddlewares) are built with theirs in place of nil. Invalid
// YAML fails Add.
func WithMiddlewareConfigs(defaults []byte) ServerOption {
	return func(s *server) {
		s.middlewareConfigs = defaults
	}
}
