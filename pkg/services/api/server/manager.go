package server

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// manager implements the Manager interface
type manager struct {
	name  string
	log   log.Logger
	debug bool

	servers map[string]*server // map of server name to server instance

	handlerFuncs map[handlerKey]any // an echo.HandlerFunc, or a wsHandler for a WS route
	middlewares  map[string]api.Middleware
}

// New creates a new server instance with the given options
func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		servers:      make(map[string]*server),
		handlerFuncs: make(map[handlerKey]any),
		middlewares:  make(map[string]api.Middleware),
	}
	m.applyOptions(opts...)
	m.setDefaults()
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
	return m.name
}

// Dependencies is nil on purpose. To a supervisor a dependency is more than
// an ordering: a dependency that fails its init keeps the dependent's Init
// from ever running, and dependencies' failures roll up into the dependent's
// healthcheck. The listener - the health listener with it - must come up
// even when a service behind some route did not, so those routes answer
// with errors while /healthz and /readyz report the truth. Starting it after
// the services is an ordering only, which the wiring gives by registering
// the server once the rest is sorted. Nor is the meter a listener carries a
// dependency - see WithMeter.
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
	switches, configs, err := s.parseMiddlewareConfigs()
	if err != nil {
		return errors.Wrapf(err, "failed to parse middleware configs for server %s", s.name)
	}
	// Each position in the chain is forced by a dependency: metrics
	// outermost of all - it reads the finished record and records (into noop
	// instruments when the listener has no meter), nothing in it can panic,
	// and outside recover a recovered panic still counts as the 500 it
	// became; then recover, because everything inside it - the user's
	// middlewares most of all - may panic; then the user's server-level
	// middlewares, in WithMiddlewares order; logger wrapping info, whose
	// records it reads; error innermost, normalizing failures closest to the
	// work.
	//
	// The user slot sits where it does because the lifecycle trio admits no
	// insertion: between logger and info a short-circuiting middleware would
	// leave logger reading records info never wrote, masking the real error,
	// and inside info is just a route middleware with extra steps - router.yaml
	// already expresses that. Ahead of the trio is the one position router.yaml
	// cannot express: every request, matched or not - which is why an app's
	// cors middleware belongs here, answering preflight requests that match no
	// route. A request rejected in this slot still gets its error normalized
	// and printed by the errorHandler fallback.
	mw := newMiddleware(s)
	funcs := []echo.MiddlewareFunc{mw.Metrics, mw.Recover}
	// Server-level middlewares are standard api.Middlewares. WithMiddlewares
	// declares the roster and its order; the server's middleware configs
	// activate each entry - a name with no entry stays dormant, `name:` or
	// `name: true` enables with no config, a block enables and configures,
	// and `name: false` switches it off. One that returns no function has
	// declined attachment.
	for _, umw := range s.middlewares {
		raw, configured := configs[umw.Name()]
		enabled, switched := switches[umw.Name()]
		if !switched {
			enabled = configured
		}
		fn, err := umw.Func(enabled, raw)
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
	s.resetWebSocket()
	return nil
}

// Add adds a new echo server instance with the given configuration
func (m *manager) Add(name string, opts ...ServerOption) error {
	// a second Add under one name would silently replace a listener that may
	// already be serving - the same Conflict RegisterMiddlewares raises
	if _, exists := m.servers[name]; exists {
		return errors.Conflict.Newf("server %s already added", name)
	}
	s := &server{
		name:     name,
		log:      m.log,
		groups:   make(map[handlerKey]*handlerGroupConfig),
		handlers: make(map[handlerKey]*handlerConfig),
	}
	s.applyOptions(opts...)
	if s.endpoint == nil {
		return errors.Newf("server must have a valid endpoint")
	}
	// once per instance: the provider's identity never changes, so the
	// rebuilt echo a restart performs keeps recording into the same series
	inst, err := s.newInstruments()
	if err != nil {
		return errors.Wrapf(err, "server %s: failed to build metrics instruments", name)
	}
	s.inst = inst
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

// registerRouter loads the router's configuration - one handler group per
// YAML document - and resolves every handler it declares. A mistake anywhere
// fails here, naming the router and the group: registration is where a
// router's config is checked, so the wiring fails startup on it.
func (m *manager) registerRouter(router api.Router) ([]*handlerGroupConfig, error) {
	// Get embedded router.yaml configuration
	data := router.Config()
	if len(data) == 0 {
		return nil, errors.Newf("router %s has empty config", router.Name())
	}
	groups, err := parseRouterConfig(data)
	if err != nil {
		return nil, errors.Wrapf(err, "router %s: failed to parse config", router.Name())
	}
	// Get handler functions from router
	handlers := router.Handlers()
	if handlers == nil {
		return nil, errors.Newf("router %s: Handlers() returned nil", router.Name())
	}
	count := 0
	for _, group := range groups {
		if err := m.resolveHandlers(group, handlers); err != nil {
			return nil, errors.Wrapf(err, "router %s, handler group %s", router.Name(), group.Prefix)
		}
		count += len(group.Handlers)
	}
	m.log.Debugf("registered router %s with %d handler(s) in %d group(s)", router.Name(), count, len(groups))
	return groups, nil
}

// resolveHandlers binds each handler a group declares to its function. A
// route is one server, method and path wherever it is declared: a second
// declaration - in another group of the same router or in another router -
// would silently replace the first, so it fails registration instead.
func (m *manager) resolveHandlers(group *handlerGroupConfig, handlers map[string]any) error {
	for _, handler := range group.Handlers {
		handler.Method = strings.ToUpper(handler.Method)
		if !validHTTPMethod(handler.Method) {
			return errors.Newf("invalid HTTP method %q for handler %s", handler.Method, handler.Func)
		}
		fn, ok := handlers[handler.Func]
		if !ok {
			return errors.NotImplemented.Newf("handler function %s not found in router.Handlers()", handler.Func)
		}
		key := newHandlerKey(group, handler)
		if _, taken := m.handlerFuncs[key]; taken {
			return errors.Conflict.Newf("route %s is already registered", key)
		}
		ws := false
		switch f := fn.(type) {
		case echo.HandlerFunc:
		case func(echo.Context) error:
			fn = echo.HandlerFunc(f)
		case wsHandler:
			ws = true
		default:
			return errors.Newf("handler %s has unsupported signature", handler.Func)
		}
		if ws && handler.Method != api.MethodWS {
			return errors.Newf("handler %s has WebSocket signature but method is %s", handler.Func, handler.Method)
		}
		if !ws && handler.Method == api.MethodWS {
			return errors.Newf("handler %s declared as WS but signature is not WebSocket", handler.Func)
		}
		m.handlerFuncs[key] = fn
	}
	return nil
}

// RegisterRouters registers one or more routers with the server
func (m *manager) RegisterRouters(routers ...api.Router) error {
	for _, r := range routers {
		// Register router and get its handler groups
		groups, err := m.registerRouter(r)
		if err != nil {
			return err
		}
		for _, g := range groups {
			// Determine which server to use
			serverName := g.Server
			if serverName == "" {
				return errors.Newf("router %s: server name not specified in handler group %q", r.Name(), g.Prefix)
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

	fn, ok := m.handlerFuncs[key]
	if !ok {
		return nil
	}

	switch h.Method {
	case api.MethodWS:
		// WebSocket handshake comes in as HTTP GET; webSocket upgrades it.
		group.Add(http.MethodGet, routePath, m.webSocket(s, fn.(wsHandler)), mwfuncs...)
	case api.MethodAny:
		group.Any(routePath, fn.(echo.HandlerFunc), mwfuncs...)
	default:
		group.Add(h.Method, routePath, fn.(echo.HandlerFunc), mwfuncs...)
	}
	return nil
}

// collectMiddlewares resolves the handler's and group's middleware refs into
// functions for one route. Handler refs come first - they wrap outermost - and
// a name the handler already claimed is skipped at group level, so a handler's
// entry overrides the group's rather than stacking a second run. The switch
// and the config resolve independently through resolveMiddleware; each ref
// costs one Func call, at registration, so a bad config fails startup.
func (m *manager) collectMiddlewares(s *server, h *handlerConfig, g *handlerGroupConfig) ([]echo.MiddlewareFunc, error) {
	groupRefs := make(map[string]*middlewareConfig, len(g.Middlewares))
	for _, ref := range g.Middlewares {
		if _, ok := groupRefs[ref.Name]; !ok {
			groupRefs[ref.Name] = ref
		}
	}
	serverSwitches, serverConfigs, err := s.parseMiddlewareConfigs()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse middleware configs for server %s", s.name)
	}

	// One pass, handler refs first so they wrap outermost and shadow the
	// group's entries. The candidate chain is the same for both scopes: a
	// group ref's own config IS groupConfigs[name], so walking it twice is
	// harmless.
	refs := make([]*middlewareConfig, 0, len(h.Middlewares)+len(g.Middlewares))
	refs = append(refs, h.Middlewares...)
	refs = append(refs, g.Middlewares...)

	mwfuncs := make([]echo.MiddlewareFunc, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		mw, ok := m.middlewares[ref.Name]
		if !ok {
			// Checked before the switch: turning off a middleware that does
			// not exist is a typo, not a no-op.
			return nil, errors.NotImplemented.Newf("middleware %s not found", ref.Name)
		}
		enabled, cfg := resolveMiddleware(ref, groupRefs[ref.Name], serverSwitches, serverConfigs)
		fn, err := mw.Func(enabled, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "middleware %s failed to build for %s %s", ref.Name, h.Method, h.Path)
		}
		if fn == nil {
			// Attachment declined - switched off, or the middleware's own
			// choice; the route serves as if the middleware were absent.
			continue
		}
		mwfuncs = append(mwfuncs, fn)
	}
	return mwfuncs, nil
}

// resolveMiddleware walks a route attachment's chain - handler entry, group
// entry, server mapping, most specific first - already split by parsing:
// the first switch decides enabled (defaulting to true, since the ref's
// presence is the intent), the first config wins, independently.
func resolveMiddleware(h, g *middlewareConfig, serverSwitches map[string]bool, serverConfigs map[string][]byte) (enabled bool, cfg []byte) {
	enabled, decided := true, false
	for _, ref := range []*middlewareConfig{h, g} {
		if ref == nil {
			continue
		}
		if ref.Enabled != nil && !decided {
			enabled, decided = *ref.Enabled, true
		}
		if ref.Config != nil && cfg == nil {
			cfg = ref.Config
		}
	}
	if b, ok := serverSwitches[h.Name]; ok && !decided {
		enabled = b
	}
	if cfg == nil {
		cfg = serverConfigs[h.Name]
	}
	return enabled, cfg
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
		go s.serve()
	}
	return nil
}

// Stop shuts the servers down, waiting for the requests going on until ctx ends; then it closes
// their connections, though a handler that ignores its request's context runs on. WebSocket
// connections are closed going away, and their handlers waited for the same way.
func (m *manager) Stop(ctx context.Context) error {
	var errs []error
	for _, s := range m.servers {
		if err := s.stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Combine(errs...)
}
