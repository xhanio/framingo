package server

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// server holds an echo instance with its configuration
type server struct {
	endpoint       *api.Endpoint
	tlsConfig      *api.ServerTLS
	throttleConfig *api.ThrottleConfig
	echo           *echo.Echo
}

// manager implements the Manager interface
type manager struct {
	name string
	log  log.Logger

	servers map[string]*server // map of server name to server instance

	groups   map[string]*api.HandlerGroup
	handlers map[string]*api.Handler

	handlerFuncs    map[string]echo.HandlerFunc
	middlewareFuncs map[string]echo.MiddlewareFunc

	sync.Mutex // lock for rate limiters
	limits     map[string]*rate.Limiter
}

// New creates a new server instance with the given options
func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		servers:         make(map[string]*server),
		groups:          make(map[string]*api.HandlerGroup),
		handlers:        make(map[string]*api.Handler),
		handlerFuncs:    make(map[string]echo.HandlerFunc),
		middlewareFuncs: make(map[string]echo.MiddlewareFunc),
		limits:          make(map[string]*rate.Limiter),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *manager) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = m.ErrorHandler
	return e
}

// ============================================================================
// Service Interface Implementation
// ============================================================================

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Init() error {
	return nil
}

// AddServer adds a new echo server instance with the given configuration
func (m *manager) AddServer(name string, opts ...ServerOption) {
	s := &server{}
	for _, opt := range opts {
		opt(s)
	}

	e := m.newEcho()
	e.Use(
		m.Recover,
		m.Logger,
		m.Info,
		m.Error,
		m.Throttle,
	)
	s.echo = e
	m.servers[name] = s
}

// ============================================================================
// Router Registration
// ============================================================================

// registerRouter loads the router's configuration and registers its handlers
func (m *manager) registerRouter(router api.Router) (*api.HandlerGroup, error) {
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
	var group *api.HandlerGroup
	if err := yaml.Unmarshal(data, &group); err != nil {
		return nil, errors.Wrapf(err, "failed to parse router config")
	}
	if group == nil {
		return nil, errors.Newf("http configuration not found in router.yaml")
	}
	// Get handler functions from router
	handlers := router.Handlers()
	if handlers == nil {
		return nil, errors.Newf("router.Handlers() returned nil")
	}

	// Determine which server to use
	serverName := group.Server
	if serverName == "" {
		serverName = "http"
	}

	// Get the server for this server to determine endpoint path
	s, ok := m.servers[serverName]
	if !ok {
		return nil, errors.Newf("server %s not found, please call AddServer first", serverName)
	}

	// Get endpoint path from the server
	var endpointPath string
	if s.endpoint != nil {
		endpointPath = s.endpoint.Path
	}

	// Register each handler function
	for _, handler := range group.Handlers {
		handlerFunc, ok := handlers[handler.Func]
		if !ok {
			return nil, errors.NotImplemented.Newf("handler function %s not found in router.Handlers()", handler.Func)
		}
		key := api.HandlerKey(endpointPath, group, handler)
		m.handlerFuncs[key] = handlerFunc
	}

	m.log.Debugf("registered router %s with %d handlers", router.Name(), len(group.Handlers))
	return group, nil
}

// RegisterRouters registers one or more routers with the server
func (m *manager) RegisterRouters(routers ...api.Router) error {
	for _, r := range routers {
		// Register router and get handler group
		g, err := m.registerRouter(r)
		if err != nil {
			return err
		}

		// Determine which server to use (from group config or http)
		serverName := g.Server
		if serverName == "" {
			serverName = "http"
		}

		// Get the echo instance for this server
		s, ok := m.servers[serverName]
		if !ok {
			return errors.Newf("server %s not found, please call AddServer first", serverName)
		}

		// Create echo group with API prefix
		var prefix string
		if s.endpoint != nil {
			prefix = path.Join(s.endpoint.Path, g.Prefix)
		}
		if prefix == "" {
			prefix = api.DefaultAPIPrefix
		}
		group := s.echo.Group(prefix)

		// Register each handler with middlewares
		for _, h := range g.Handlers {
			// Collect middlewares (handler-specific + group-level)
			mwfuncs, err := m.collectMiddlewares(h, g)
			if err != nil {
				return err
			}

			// Register route with Echo
			if hf, ok := m.handlerFuncs[api.HandlerKey(prefix, g, h)]; ok {
				group.Add(h.Method, h.Path, hf, mwfuncs...)
			}

			// Store handler metadata for request lookup
			key := api.HandlerKey(prefix, g, h)
			m.groups[key] = g
			m.handlers[key] = h
		}
	}
	return nil
}

func (m *manager) Routers() (map[string]*api.HandlerGroup, map[string]*api.Handler) {
	return m.groups, m.handlers
}

// collectMiddlewares gathers handler-specific and group-level middlewares
func (m *manager) collectMiddlewares(h *api.Handler, g *api.HandlerGroup) ([]echo.MiddlewareFunc, error) {
	var mwfuncs []echo.MiddlewareFunc

	// Collect handler-specific middlewares
	for _, name := range h.Middlewares {
		if mw, ok := m.middlewareFuncs[name]; ok {
			mwfuncs = append(mwfuncs, mw)
		} else {
			return nil, errors.NotImplemented.Newf("middleware %s not found", name)
		}
	}

	// Collect group-level middlewares
	for _, name := range g.Middlewares {
		if mw, ok := m.middlewareFuncs[name]; ok {
			mwfuncs = append(mwfuncs, mw)
		} else {
			return nil, errors.NotImplemented.Newf("middleware %s not found", name)
		}
	}

	return mwfuncs, nil
}

// RegisterMiddlewares registers middlewares with the server
func (m *manager) RegisterMiddlewares(middlewares ...api.Middleware) {
	for _, mw := range middlewares {
		m.middlewareFuncs[mw.Name()] = mw.Func
	}
}

// ============================================================================
// Server Lifecycle
// ============================================================================

// startServer starts a single HTTP or HTTPS server
func (m *manager) startServer(name string, s *server) error {
	if s.endpoint == nil {
		return nil
	}

	if s.tlsConfig == nil {
		m.log.Infof("serves http [%s] on %s", name, s.endpoint.String())
		return s.echo.Start(s.endpoint.Address())
	}

	s.echo.TLSServer = &http.Server{
		Addr:      s.endpoint.Address(),
		TLSConfig: s.tlsConfig.AsConfig(),
	}
	m.log.Infof("serves https [%s] on %s", name, s.endpoint.String())
	return s.echo.StartServer(s.echo.TLSServer)
}

// Start starts all servers in goroutines
func (m *manager) Start(ctx context.Context) error {
	for name, s := range m.servers {
		go func(n string, srv *server) {
			if err := m.startServer(n, srv); err != nil {
				m.log.Debugf("server %s start error: %v", n, err)
			}
		}(name, s)
	}
	return nil
}

// Stop gracefully shuts down all servers
func (m *manager) Stop(wait bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for name, s := range m.servers {
		if err := s.echo.Shutdown(ctx); err != nil {
			m.log.Errorf("failed to stop server %s: %v", name, err)
			return errors.Wrap(err)
		}
	}
	return nil
}
