package server

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/utils/log"
)

var _ Server = (*server)(nil)

// server holds an echo instance with its configuration and implements model.APIServer interface
type server struct {
	name string
	log  log.Logger

	endpoint    *api.Endpoint
	tlsConfig   *api.ServerTLS
	middlewares []api.Middleware
	echo        *echo.Echo

	groups   map[handlerKey]*handlerGroupConfig
	handlers map[handlerKey]*handlerConfig
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
