package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// server implements the Server interface
type server struct {
	name string
	log  log.Logger

	endpoint       *api.Endpoint
	tlsConfig      *api.ServerTLS
	throttleConfig *api.ThrottleConfig

	core *echo.Echo

	groups   map[string]*api.HandlerGroup
	handlers map[string]*api.Handler

	handlerFuncs    map[string]echo.HandlerFunc
	middlewareFuncs map[string]echo.MiddlewareFunc

	sync.Mutex // lock for rate limiters
	limits     map[string]*rate.Limiter
}

// New creates a new server instance with the given options
func New(opts ...Option) Server {
	return newServer(opts...)
}

func newServer(opts ...Option) *server {
	s := &server{
		groups:          make(map[string]*api.HandlerGroup),
		handlers:        make(map[string]*api.Handler),
		handlerFuncs:    make(map[string]echo.HandlerFunc),
		middlewareFuncs: make(map[string]echo.MiddlewareFunc),
		limits:          make(map[string]*rate.Limiter),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = log.Default
	}
	return s
}

func (s *server) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = s.ErrorHandler
	return e
}

// ============================================================================
// Service Interface Implementation
// ============================================================================

func (s *server) Name() string {
	if s.name == "" {
		s.name = path.Join(reflectutil.Locate(s))
	}
	return s.name
}

func (s *server) Dependencies() []common.Service {
	return nil
}

func (s *server) Init() error {
	s.core = s.newEcho()
	s.core.Use(
		s.Recover,
		s.Logger,
		s.Info,
		s.Error,
		s.Throttle,
	)
	return nil
}

func (s *server) Endpoint() *api.Endpoint {
	return s.endpoint
}

// ============================================================================
// Router Registration
// ============================================================================

// registerRouter loads the router's configuration and registers its handlers
func (s *server) registerRouter(router api.Router) (*api.HandlerGroup, error) {
	// Get router package path to locate router.yaml
	pkgPath, _ := reflectutil.Locate(router)

	// Convert package path to file system path
	// e.g., "github.com/xhanio/framingo/pkg/routers/example" -> "pkg/routers/example"
	routerDir := pkgPath
	if idx := strings.Index(pkgPath, "/pkg/"); idx != -1 {
		routerDir = pkgPath[idx+1:]
	}

	// Load router.yaml configuration
	configPath := filepath.Join(routerDir, "router.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read router config at %s", configPath)
	}

	// Parse YAML config
	var config struct {
		HTTP *api.HandlerGroup `yaml:"http"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrapf(err, "failed to parse router config")
	}

	if config.HTTP == nil {
		return nil, errors.Newf("http configuration not found in router.yaml")
	}

	group := config.HTTP

	// Get handler functions from router
	handlers := router.Handlers()
	if handlers == nil {
		return nil, errors.Newf("router.Handlers() returned nil")
	}

	// Register each handler function
	for _, handler := range group.Handlers {
		handlerFunc, ok := handlers[handler.Func]
		if !ok {
			return nil, errors.NotImplemented.Newf("handler function %s not found in router.Handlers()", handler.Func)
		}

		key := api.HandlerKey(group, handler)
		s.handlerFuncs[key] = handlerFunc
	}

	s.log.Debugf("registered router %s with %d handlers", router.Name(), len(group.Handlers))
	return group, nil
}

// RegisterRouters registers one or more routers with the server
func (s *server) RegisterRouters(routers ...api.Router) error {
	for _, r := range routers {
		// Register router and get handler group
		g, err := s.registerRouter(r)
		if err != nil {
			return err
		}

		// Create echo group with API prefix
		var prefix string
		if s.endpoint != nil {
			prefix = path.Join(s.endpoint.Path, g.Prefix)
		}
		if prefix == "" {
			prefix = api.DefaultAPIPrefix
		}
		group := s.core.Group(prefix)

		// Register each handler with middlewares
		for _, h := range g.Handlers {
			// Collect middlewares (handler-specific + group-level)
			mwfuncs, err := s.collectMiddlewares(h, g)
			if err != nil {
				return err
			}

			// Register route with Echo
			if hf, ok := s.handlerFuncs[api.HandlerKey(g, h)]; ok {
				group.Add(h.Method, h.Path, hf, mwfuncs...)
			}

			// Store handler metadata for request lookup
			fullPath := path.Join(prefix, h.Path)
			key := fmt.Sprintf("<%s>%s", h.Method, fullPath)
			s.groups[key] = g
			s.handlers[key] = h
		}
	}
	return nil
}

// collectMiddlewares gathers handler-specific and group-level middlewares
func (s *server) collectMiddlewares(h *api.Handler, g *api.HandlerGroup) ([]echo.MiddlewareFunc, error) {
	var mwfuncs []echo.MiddlewareFunc

	// Collect handler-specific middlewares
	for _, name := range h.Middlewares {
		if mw, ok := s.middlewareFuncs[name]; ok {
			mwfuncs = append(mwfuncs, mw)
		} else {
			return nil, errors.NotImplemented.Newf("middleware %s not found", name)
		}
	}

	// Collect group-level middlewares
	for _, name := range g.Middlewares {
		if mw, ok := s.middlewareFuncs[name]; ok {
			mwfuncs = append(mwfuncs, mw)
		} else {
			return nil, errors.NotImplemented.Newf("middleware %s not found", name)
		}
	}

	return mwfuncs, nil
}

// RegisterMiddlewares registers middlewares with the server
func (s *server) RegisterMiddlewares(middlewares ...api.Middleware) {
	for _, mw := range middlewares {
		s.middlewareFuncs[mw.Name()] = mw.Func
	}
}

// ============================================================================
// Server Lifecycle
// ============================================================================

// start starts the HTTP or HTTPS server
func (s *server) start() error {
	if s.endpoint == nil {
		return nil
	}

	if s.tlsConfig == nil {
		s.log.Infof("serves http on %s", s.endpoint.String())
		return s.core.Start(s.endpoint.Address())
	}

	s.core.TLSServer = &http.Server{
		Addr:      s.endpoint.Address(),
		TLSConfig: s.tlsConfig.AsConfig(),
	}
	s.log.Infof("serves https on %s", s.endpoint.String())
	return s.core.StartServer(s.core.TLSServer)
}

// Start starts the server in a goroutine
func (s *server) Start(ctx context.Context) error {
	go func() {
		if err := s.start(); err != nil {
			s.log.Debugf("server start error: %v", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the server
func (s *server) Stop(wait bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.core.Shutdown(ctx); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
