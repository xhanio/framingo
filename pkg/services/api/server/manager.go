package server

import (
	"context"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// manager implements the Manager interface
type manager struct {
	name  string
	log   log.Logger
	debug bool

	servers map[string]*server // map of server name to server instance

	handlerFuncs    map[api.HandlerKey]echo.HandlerFunc
	wsHandlerFuncs  map[api.HandlerKey]api.WebSocketHandlerFunc
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
		log:             log.Default,
		servers:         make(map[string]*server),
		handlerFuncs:    make(map[api.HandlerKey]echo.HandlerFunc),
		wsHandlerFuncs:  make(map[api.HandlerKey]api.WebSocketHandlerFunc),
		middlewareFuncs: make(map[string]echo.MiddlewareFunc),
		limits:          make(map[string]*rate.Limiter),
	}
	m.apply(opts...)
	return m
}

func (m *manager) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
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

func (m *manager) Init(ctx context.Context) error {
	return nil
}

// Add adds a new echo server instance with the given configuration
func (m *manager) Add(name string, opts ...ServerOption) error {
	s := &server{
		name:     name,
		log:      m.log,
		groups:   make(map[api.HandlerKey]*api.HandlerGroup),
		handlers: make(map[api.HandlerKey]*api.Handler),
	}
	s.apply(opts...)
	if s.endpoint == nil {
		return errors.Newf("server must have a valid endpoint")
	}
	mw := newMiddleware(s)
	e := m.newEcho()
	e.HTTPErrorHandler = s.errorHandler
	e.Pre(middleware.RemoveTrailingSlash())
	var middlewares []echo.MiddlewareFunc
	// Apply CORS middleware in debug mode
	if m.debug {
		middlewares = append(middlewares, mw.CORS())
	}
	middlewares = append(middlewares,
		mw.Recover,
		mw.Logger,
		mw.Info,
		mw.Error,
		mw.Throttle,
	)
	e.Use(middlewares...)
	s.echo = e
	m.servers[name] = s
	return nil
}

// Get returns the Server interface for the given server name
func (m *manager) Get(name string) (Server, error) {
	s, ok := m.servers[name]
	if !ok {
		return nil, errors.Newf("server %s not found", name)
	}
	return s, nil
}

// List returns all registered servers
func (m *manager) List() []Server {
	servers := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	return servers
}

// ============================================================================
// Router Registration
// ============================================================================

// registerRouter loads the router's configuration and registers its handlers
func (m *manager) registerRouter(router api.Router) (*api.HandlerGroup, error) {
	// Get embedded router.yaml configuration
	data := router.Config()
	if len(data) == 0 {
		return nil, errors.Newf("router %s has empty config", router.Name())
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
	// Register each handler function
	for _, handler := range group.Handlers {
		handler.Method = strings.ToUpper(handler.Method)
		if !validHTTPMethod(handler.Method) {
			return nil, errors.Newf("invalid HTTP method %q for handler %s", handler.Method, handler.Func)
		}
		fn, ok := handlers[handler.Func]
		if !ok {
			return nil, errors.NotImplemented.Newf("handler function %s not found in router.Handlers()", handler.Func)
		}
		key := api.NewHandlerKey(group, handler)
		if handler.Method == api.MethodWS {
			switch f := fn.(type) {
			case api.WebSocketHandlerFunc:
				m.wsHandlerFuncs[key] = f
			case func(context.Context, *websocket.Conn) error:
				m.wsHandlerFuncs[key] = f
			default:
				return nil, errors.Newf("handler %s declared as WS but is not api.WebSocketHandlerFunc", handler.Func)
			}
		} else {
			switch f := fn.(type) {
			case echo.HandlerFunc:
				m.handlerFuncs[key] = f
			case func(echo.Context) error:
				m.handlerFuncs[key] = f
			default:
				return nil, errors.Newf("handler %s must be echo.HandlerFunc", handler.Func)
			}
		}
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
		// Determine which server to use
		serverName := g.Server
		if serverName == "" {
			return errors.Newf("server name not specified in router configuration")
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
			key := api.NewHandlerKey(g, h)
			// Normalize root path "/" to "" so the route registers at the
			// group prefix without a trailing slash. Combined with the
			// RemoveTrailingSlash pre-middleware, both /prefix and /prefix/
			// resolve to the same handler.
			routePath := strings.TrimSuffix(h.Path, "/")
			m.log.Infof("register handler %s %s%s", h.Method, prefix, h.Path)

			var hf echo.HandlerFunc
			switch h.Method {
			case api.MethodWS:
				wsFunc, ok := m.wsHandlerFuncs[key]
				if !ok {
					continue
				}
				hf = m.wrapWebSocket(wsFunc)
			default:
				var ok bool
				hf, ok = m.handlerFuncs[key]
				if !ok {
					continue
				}
			}

			switch h.Method {
			case api.MethodWS:
				group.Add(http.MethodGet, routePath, hf, mwfuncs...)
			case api.MethodAny:
				group.Any(routePath, hf, mwfuncs...)
			default:
				group.Add(h.Method, routePath, hf, mwfuncs...)
			}
			s.groups[key] = g
			s.handlers[key] = h
		}
	}
	return nil
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

// wrapWebSocket wraps a WebSocketHandlerFunc into an echo.HandlerFunc
// that upgrades the HTTP connection and delegates to the WS handler.
func (m *manager) wrapWebSocket(fn api.WebSocketHandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
			InsecureSkipVerify: m.debug,
		})
		if err != nil {
			return errors.BadRequest.Wrap(err)
		}

		// Once upgraded, the HTTP response is hijacked — errors cannot be
		// returned to Echo. Handle closure directly.
		ctx := c.Request().Context()
		if err = fn(ctx, conn); err != nil {
			m.log.Error(errors.Wrap(err))
			conn.Close(websocket.StatusInternalError, err.Error())
		} else {
			conn.Close(websocket.StatusNormalClosure, "")
		}
		return nil
	}
}

// RegisterMiddlewares registers middlewares with the server
func (m *manager) RegisterMiddlewares(middlewares ...api.Middleware) error {
	for _, mw := range middlewares {
		name := mw.Name()
		if _, exists := m.middlewareFuncs[name]; exists {
			return errors.Conflict.Newf("middleware %s already registered", name)
		}
		m.middlewareFuncs[name] = mw.Func
	}
	return nil
}

// ============================================================================
// Lifecycle
// ============================================================================

// Start starts all servers in goroutines
func (m *manager) Start(ctx context.Context) error {
	for _, s := range m.servers {
		go func(srv *server) {
			if err := srv.start(); err != nil {
				srv.log.Debugf("server %s start error: %v", srv.name, err)
			}
		}(s)
	}
	return nil
}

// Stop gracefully shuts down all servers
func (m *manager) Stop(wait bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range m.servers {
		if err := s.stop(ctx); err != nil {
			return errors.Wrap(err)
		}
	}
	return nil
}
