package server

import (
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

// ServerOption configures a server (echo server instance)
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

func WithTLS(cert certutil.CertBundle, auth bool) ServerOption {
	return func(s *server) {
		s.tlsConfig = &api.ServerTLS{
			CertBundle:  cert,
			AuthEnabled: auth,
		}
	}
}

// WithMeter turns on the server's HTTP metrics - request duration, active
// requests and body sizes, recorded through the given handle for every
// request this server instance serves, matched or not. A server without
// one records into noop instruments and nothing lands - the case for an app
// that wires no metrics service; with one, the metrics service's own
// enabled switch decides what exports. The meter is deliberately not a dependency
// of the api server: the metrics service has liveness only, and a
// collector outage or an unconfigured exporter must degrade metrics, never
// the listener.
func WithMeter(meter model.Meter) ServerOption {
	return func(s *server) {
		s.meter = meter
	}
}

// WithMiddlewares installs middlewares on the server itself, running on every
// request - matched or not - inside the built-in recover but ahead of the
// rest of the lifecycle, in the order given. This is the position router.yaml
// cannot express, which is where an app's cors middleware belongs: preflight
// requests match no route. Each is built with the server's middleware config
// under its name and survives the echo rebuild a restart performs.
// RequestInfo does not exist yet at this position; a middleware that reads it
// belongs in router.yaml instead.
func WithMiddlewares(mws ...api.Middleware) ServerOption {
	return func(s *server) {
		s.middlewares = append(s.middlewares, mws...)
	}
}

// WithMiddlewareConfigs sets the server's default middleware configs, a plain
// mapping of middleware name to value - unlike a router.yaml middleware list,
// which stays a sequence because attachment order matters there:
//
//	cors: true
//	throttle:
//	  rps: 100.0
//	  burst_size: 200
//
// A boolean value is a switch, not config: cors: false disables the cors
// middleware wherever nothing more specific overrides it, and cors: true
// enables it with no config. A block is config and resolves
// most-specific-first into Func's config argument: a handler entry's own
// block, else its group entry's, else this mapping's, else nil.
//
// For server-level middlewares (WithMiddlewares) this mapping is also the
// activation: no entry leaves one dormant, a null or true entry enables it,
// a block enables and configures, false switches it off. The lifecycle
// built-ins (metrics, recover, logger, info, error) take no config and claim no
// names - every name here belongs to the app's middlewares. Invalid YAML
// fails Add.
func WithMiddlewareConfigs(defaults []byte) ServerOption {
	return func(s *server) {
		s.middlewareConfigs = defaults
	}
}
