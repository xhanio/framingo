package server

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// server holds an echo instance with its configuration and implements Server interface
type server struct {
	name string
	log  log.Logger

	endpoint       *api.Endpoint
	tlsConfig      *api.ServerTLS
	throttleConfig *api.ThrottleConfig
	echo           *echo.Echo

	groups   map[string]*api.HandlerGroup
	handlers map[string]*api.Handler
}

func (s *server) Name() string {
	return s.name
}

// Routers returns all handler groups and handlers for this server
func (s *server) Routers() (map[string]*api.HandlerGroup, map[string]*api.Handler) {
	return s.groups, s.handlers
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
