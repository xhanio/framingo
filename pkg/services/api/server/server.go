package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

var _ Server = (*server)(nil)

type server struct {
	sync.Mutex

	name string
	log  log.Logger

	endpoint          *api.Endpoint
	tlsConfig         *api.ServerTLS
	middlewares       []api.Middleware
	middlewareConfigs []byte
	echo              *echo.Echo

	// meter may be nil; inst never is (noops without a meter)
	meter model.Meter
	inst  *instruments

	groups   map[handlerKey]*handlerGroupConfig
	handlers map[handlerKey]*handlerConfig

	err error // why the serve loop exited; nil while serving or after a graceful shutdown

	// Shutdown neither closes nor waits for upgraded connections: wsCtx (one per echo)
	// ends when stop begins, and wsWG counts handlers across echos for stop to wait on
	wsCtx    context.Context
	wsCancel context.CancelFunc
	wsWG     syncutil.WaitGroup
}

func (s *server) serve() {
	s.Lock()
	s.err = nil
	s.Unlock()
	err := s.start()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	s.Lock()
	s.err = err
	s.Unlock()
	s.log.Warnf("server %s stopped serving: %v", s.name, err)
}

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

func (s *server) stop(ctx context.Context) error {
	s.Lock()
	s.wsCancel()
	s.Unlock()
	err := s.echo.Shutdown(ctx)
	if err == nil {
		if err := s.wsWG.Wait(ctx); err != nil {
			return errors.Wrapf(err, "server %s stopped with websocket handlers still running", s.name)
		}
		return nil
	}
	if ctx.Err() != nil {
		if cerr := s.echo.Close(); cerr != nil {
			s.log.Errorf("failed to close server %s: %v", s.name, cerr)
		}
		return errors.Wrapf(err, "server %s closed with requests still going", s.name)
	}
	s.log.Errorf("failed to stop server %s: %v", s.name, err)
	return errors.Wrap(err)
}

// a null entry is a present key in configs with nil bytes
func (s *server) parseMiddlewareConfigs() (switches map[string]bool, configs map[string][]byte, err error) {
	if s.middlewareConfigs == nil {
		return nil, nil, nil
	}
	var entries map[string]yaml.Node
	if err := yaml.Unmarshal(s.middlewareConfigs, &entries); err != nil {
		return nil, nil, errors.Wrapf(err, "invalid middleware configs")
	}
	switches = make(map[string]bool, len(entries))
	configs = make(map[string][]byte, len(entries))
	for name, node := range entries {
		enabled, raw, err := splitMiddlewareValue(&node)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "invalid middleware config %s", name)
		}
		if enabled != nil {
			switches[name] = *enabled
			continue
		}
		configs[name] = raw
	}
	return switches, configs, nil
}
