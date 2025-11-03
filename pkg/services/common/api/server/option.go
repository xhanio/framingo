package server

import (
	"golang.org/x/time/rate"

	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*server)

func WithLogger(logger log.Logger) Option {
	return func(s *server) {
		s.log = logger
	}
}

func WithEndpoint(host string, port uint, prefix string) Option {
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

func WithCert(cert certutil.CertBundle) Option {
	return func(s *server) {
		s.tlsConfig = &api.ServerTLS{
			CertBundle:  cert,
			AuthEnabled: true,
		}
	}
}

func WithThrottle(rps float64, burstSize int) Option {
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
