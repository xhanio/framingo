package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/log"
)

var _ Server = (*server)(nil)

// server holds an echo instance with its configuration and implements model.APIServer interface
type server struct {
	sync.Mutex

	name string
	log  log.Logger

	endpoint          *api.Endpoint
	tlsConfig         *api.ServerTLS
	middlewares       []api.Middleware
	middlewareConfigs []byte
	echo              *echo.Echo

	groups   map[handlerKey]*handlerGroupConfig
	handlers map[handlerKey]*handlerConfig

	err error // why the serve loop exited; nil while serving or after a graceful shutdown
}

// serve runs the listener until it stops and remembers why. The record is
// cleared on every launch, so a rebuilt listener starts with a clean slate;
// http.ErrServerClosed is the expected return after a graceful Shutdown and
// is not worth remembering.
func (s *server) serve() {
	s.Lock()
	s.err = nil
	s.Unlock()
	err := s.start()
	if err == nil || err == http.ErrServerClosed {
		return
	}
	s.Lock()
	s.err = err
	s.Unlock()
	s.log.Warnf("server %s stopped serving: %v", s.name, err)
}

// serveError reports why the serve loop exited: nil while serving, or after
// a graceful shutdown.
func (s *server) serveError() error {
	s.Lock()
	defer s.Unlock()
	return s.err
}

func (s *server) Name() string {
	return s.name
}

func (s *server) Endpoint() *api.Endpoint {
	return s.endpoint
}

// start starts a single HTTP or HTTPS server
func (s *server) start() error {
	if s.endpoint == nil {
		return nil
	}
	if s.tlsConfig == nil {
		s.log.Infof("serves http [%s] on %s", s.name, s.endpoint.String())
		return s.echo.Start(s.endpoint.Address())
	}
	s.echo.TLSServer = &http.Server{
		Addr:      s.endpoint.Address(),
		TLSConfig: s.tlsConfig.AsConfig(),
	}
	s.log.Infof("serves https [%s] on %s", s.name, s.endpoint.String())
	return s.echo.StartServer(s.echo.TLSServer)
}

// stop gracefully shuts down the server
func (s *server) stop(ctx context.Context) error {
	if err := s.echo.Shutdown(ctx); err != nil {
		s.log.Errorf("failed to stop server %s: %v", s.name, err)
		return err
	}
	return nil
}

// parseMiddlewareConfigs resolves the raw block handed to
// WithMiddlewareConfigs into a name-to-config map. Unlike a router.yaml
// middleware list, the block is a plain mapping - defaults carry no order:
//
//	cors: true
//	throttle:
//	  rps: 100.0
//	  burst_size: 200
//
// A name mapped to null carries a nil config. Called wherever the configs are
// consumed; Add surfaces an invalid block before the server exists.
func (s *server) parseMiddlewareConfigs() (map[string][]byte, error) {
	if s.middlewareConfigs == nil {
		return nil, nil
	}
	var entries map[string]yaml.Node
	if err := yaml.Unmarshal(s.middlewareConfigs, &entries); err != nil {
		return nil, errors.Wrapf(err, "invalid middleware configs")
	}
	result := make(map[string][]byte, len(entries))
	for name, node := range entries {
		raw, err := configBytes(&node)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid middleware config %s", name)
		}
		result[name] = raw
	}
	return result, nil
}
