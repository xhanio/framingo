package server

import (
	"context"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"
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

	handlerFuncs map[handlerKey]echo.HandlerFunc
	middlewares  map[string]api.Middleware
}

// New creates a new server instance with the given options
func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		log:          log.Default,
		servers:      make(map[string]*server),
		handlerFuncs: make(map[handlerKey]echo.HandlerFunc),
		middlewares:  make(map[string]api.Middleware),
	}
	m.apply(opts...)
	return m
}

type echoValidator struct {
	v *validator.Validate
}

func (ev *echoValidator) Validate(i any) error {
	return ev.v.Struct(i)
}

func (m *manager) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = &echoValidator{v: validator.New()}
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

// Init rebuilds every server's echo instance and re-registers all routes
// from the persisted handler metadata. Required because net/http forbids
// reusing a server after Shutdown — on restart we need fresh echos.
func (m *manager) Init(ctx context.Context) error {
	for _, s := range m.servers {
		if err := m.rebuildServer(s); err != nil {
			return err
		}
	}
	return nil
}

// rebuildServer gives the server a fresh echo instance and reinstalls every
// route it holds.
func (m *manager) rebuildServer(s *server) error {
	if err := m.buildEcho(s); err != nil {
		return err
	}
	for key, h := range s.handlers {
		g := s.groups[key]
		if err := m.installHandler(s, g, h); err != nil {
			return err
		}
	}
	return nil
}

// buildEcho creates a fresh echo instance for the given server and applies
// its middleware chain. Replaces any prior s.echo.
func (m *manager) buildEcho(s *server) error {
	e := m.newEcho()
	e.HTTPErrorHandler = s.errorHandler
	e.Pre(middleware.RemoveTrailingSlash())
	configs, err := s.parseMiddlewareConfigs()
	if err != nil {
		return errors.Wrapf(err, "failed to parse middleware configs for server %s", s.name)
	}
	// Each position in the chain is forced by a dependency: recover outermost,
	// because everything inside it - the user's middlewares most of all - may
	// panic; cors next, answering preflight requests that match no route
	// before any user code runs; then the user's server-level middlewares;
	// logger wrapping info, whose records it reads; error innermost,
	// normalizing failures closest to the work.
	//
	// The user slot sits where it does because the lifecycle trio admits no
	// insertion: between logger and info a short-circuiting middleware would
	// leave logger reading records info never wrote, masking the real error,
	// and inside info is just a route middleware with extra steps - router.yaml
	// already expresses that. Ahead of the trio is the one position router.yaml
	// cannot express: every request, matched or not. A request rejected there
	// still gets its error normalized and printed by the errorHandler fallback.
	mw := newMiddleware(s)
	funcs := []echo.MiddlewareFunc{mw.Recover}
	// cors and the user's server-level middlewares are standard
	// api.Middlewares, each built with the server's config under its name;
	// one that returns no function has declined attachment.
	for _, umw := range append([]api.Middleware{&corsMiddleware{}}, s.middlewares...) {
		fn, err := umw.Func(configs[umw.Name()])
		if err != nil {
			return errors.Wrapf(err, "middleware %s failed to build for server %s", umw.Name(), s.name)
		}
		if fn == nil {
			continue
		}
		funcs = append(funcs, fn)
	}
	funcs = append(funcs, mw.Logger, mw.Info, mw.Error)
	e.Use(funcs...)
	s.echo = e
	return nil
}

// Add adds a new echo server instance with the given configuration
func (m *manager) Add(name string, opts ...ServerOption) error {
	s := &server{
		name:     name,
		log:      m.log,
		groups:   make(map[handlerKey]*handlerGroupConfig),
		handlers: make(map[handlerKey]*handlerConfig),
	}
	s.apply(opts...)
	if s.endpoint == nil {
		return errors.Newf("server must have a valid endpoint")
	}
	if err := m.buildEcho(s); err != nil {
		return err
	}
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
func (m *manager) registerRouter(router api.Router) (*handlerGroupConfig, error) {
	// Get embedded router.yaml configuration
	data := router.Config()
	if len(data) == 0 {
		return nil, errors.Newf("router %s has empty config", router.Name())
	}
	// Parse YAML config
	var group *handlerGroupConfig
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
		key := newHandlerKey(group, handler)
		switch f := fn.(type) {
		case echo.HandlerFunc:
			if handler.Method == api.MethodWS {
				return nil, errors.Newf("handler %s declared as WS but signature is not WebSocket", handler.Func)
			}
			m.handlerFuncs[key] = f
		case func(echo.Context) error:
			if handler.Method == api.MethodWS {
				return nil, errors.Newf("handler %s declared as WS but signature is not WebSocket", handler.Func)
			}
			m.handlerFuncs[key] = f
		case func(echo.Context, *websocket.Conn) error:
			if handler.Method != api.MethodWS {
				return nil, errors.Newf("handler %s has WebSocket signature but method is %s", handler.Func, handler.Method)
			}
			m.handlerFuncs[key] = m.wrapWebSocket(f)
		default:
			return nil, errors.Newf("handler %s has unsupported signature", handler.Func)
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

		for _, h := range g.Handlers {
			key := newHandlerKey(g, h)
			s.groups[key] = g
			s.handlers[key] = h
			if err := m.installHandler(s, g, h); err != nil {
				return err
			}
		}
	}
	return nil
}

// installHandler registers a single handler on the server's echo instance.
// Used by both RegisterRouters (initial wiring) and Init (rebuild on restart).
func (m *manager) installHandler(s *server, g *handlerGroupConfig, h *handlerConfig) error {
	// Create echo group with API prefix.
	// Trim trailing slash so Echo's literal prefix+path concatenation
	// doesn't produce double slashes (e.g., "/" + "/health" → "//health").
	prefix := strings.TrimSuffix(path.Join(s.endpoint.Path, g.Prefix), "/")
	group := s.echo.Group(prefix)

	mwfuncs, err := m.collectMiddlewares(s, h, g)
	if err != nil {
		return err
	}

	key := newHandlerKey(g, h)
	// Normalize root path "/" to "" so the route registers at the
	// group prefix without a trailing slash. Combined with the
	// RemoveTrailingSlash pre-middleware, both /prefix and /prefix/
	// resolve to the same handler.
	routePath := strings.TrimSuffix(h.Path, "/")
	m.log.Infof("register handler %s %s", h.Method, path.Join(prefix, h.Path))

	hf, ok := m.handlerFuncs[key]
	if !ok {
		return nil
	}

	switch h.Method {
	case api.MethodWS:
		// WebSocket handshake comes in as HTTP GET; the handler upgrades.
		group.Add(http.MethodGet, routePath, hf, mwfuncs...)
	case api.MethodAny:
		group.Any(routePath, hf, mwfuncs...)
	default:
		group.Add(h.Method, routePath, hf, mwfuncs...)
	}
	return nil
}

// collectMiddlewares resolves the handler's and group's middleware refs into
// functions for one route. Handler refs come first - they wrap outermost - and
// a name the handler already claimed is skipped at group level, so a handler's
// entry overrides the group's attachment rather than stacking a second run.
// Config resolves most-specific-first: the entry's own block, else the group
// entry's block for the same name, else the server's middleware config, else
// nil. Each ref costs one Func call, at registration, so a bad config fails
// startup.
func (m *manager) collectMiddlewares(s *server, h *handlerConfig, g *handlerGroupConfig) ([]echo.MiddlewareFunc, error) {
	groupConfigs := make(map[string][]byte, len(g.Middlewares))
	for _, ref := range g.Middlewares {
		if _, ok := groupConfigs[ref.Name]; !ok {
			groupConfigs[ref.Name] = ref.Config
		}
	}
	serverConfigs, err := s.parseMiddlewareConfigs()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse middleware configs for server %s", s.name)
	}

	mwfuncs := make([]echo.MiddlewareFunc, 0, len(h.Middlewares)+len(g.Middlewares))
	seen := make(map[string]bool, len(h.Middlewares)+len(g.Middlewares))
	install := func(name string, cfg []byte) error {
		mw, ok := m.middlewares[name]
		if !ok {
			return errors.NotImplemented.Newf("middleware %s not found", name)
		}
		fn, err := mw.Func(cfg)
		if err != nil {
			return errors.Wrapf(err, "middleware %s failed to build for %s %s", name, h.Method, h.Path)
		}
		if fn == nil {
			// Attachment declined - the route serves as if the middleware
			// were absent.
			return nil
		}
		mwfuncs = append(mwfuncs, fn)
		return nil
	}
	for _, ref := range h.Middlewares {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		cfg := ref.Config
		if cfg == nil {
			cfg = groupConfigs[ref.Name]
		}
		if cfg == nil {
			cfg = serverConfigs[ref.Name]
		}
		if err := install(ref.Name, cfg); err != nil {
			return nil, err
		}
	}
	for _, ref := range g.Middlewares {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		cfg := ref.Config
		if cfg == nil {
			cfg = serverConfigs[ref.Name]
		}
		if err := install(ref.Name, cfg); err != nil {
			return nil, err
		}
	}
	return mwfuncs, nil
}

// wrapWebSocket wraps a WebSocketHandlerFunc into an echo.HandlerFunc
// that upgrades the HTTP connection and delegates to the WS handler.
func (m *manager) wrapWebSocket(fn func(echo.Context, *websocket.Conn) error) echo.HandlerFunc {
	return func(c echo.Context) error {
		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
			InsecureSkipVerify: m.debug,
		})
		if err != nil {
			return errors.BadRequest.Wrap(err)
		}

		// Once upgraded, the HTTP response is hijacked — errors cannot be
		// returned to Echo. Handle closure directly.
		err = fn(c, conn)
		m.closeWebSocket(c.Request().Context(), conn, err)
		return nil
	}
}

// closeWebSocket handles WebSocket connection closure after the handler returns.
func (m *manager) closeWebSocket(ctx context.Context, conn *websocket.Conn, err error) {
	if err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
		return
	}
	switch status := websocket.CloseStatus(err); {
	case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
		// Client closed intentionally — not an error.
	case ctx.Err() != nil:
		conn.Close(websocket.StatusGoingAway, "server shutting down")
	default:
		m.log.Error(errors.Wrap(err))
		conn.Close(websocket.StatusInternalError, err.Error())
	}
}

// RegisterMiddlewares registers middlewares with the server. The middleware
// itself is kept, not a function: functions are minted per attachment point
// once router.yaml configs are known.
func (m *manager) RegisterMiddlewares(middlewares ...api.Middleware) error {
	for _, mw := range middlewares {
		name := mw.Name()
		if _, exists := m.middlewares[name]; exists {
			return errors.Conflict.Newf("middleware %s already registered", name)
		}
		m.middlewares[name] = mw
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
			// http.ErrServerClosed is the expected return from echo.Start
			// after a graceful Shutdown — not an error worth logging.
			if err := srv.start(); err != nil && err != http.ErrServerClosed {
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
