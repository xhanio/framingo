package server

import (
	"golang.org/x/time/rate"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

// ServerOption configures a server (echo server instance)
type ServerOption func(*server)

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

func WithThrottle(rps float64, burstSize int) ServerOption {
	return func(s *server) {
		if rps == 0 || burstSize == 0 {
			// no throttle control
			return
		}
		s.throttleConfig = &api.ThrottleConfig{
			RPS:       rate.Limit(rps),
			BurstSize: burstSize,
		}
	}
}
