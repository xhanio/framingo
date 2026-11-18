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

type manager struct {
	name  string
	log   log.Logger
	debug bool

	servers map[string]*server

	handlerFuncs map[handlerKey]any // an echo.HandlerFunc, or a wsHandler for a WS route
	middlewares  map[string]api.Middleware
}

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

// a failed tag is InvalidArgument (400); a non-struct argument is the handler's bug and stays a 500
func (ev *echoValidator) Validate(i any) error {
	err := ev.v.Struct(i)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return errors.Wrap(err)
	}
	return errors.InvalidArgument.Wrap(err)
}

func (m *manager) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = &echoValidator{v: validator.New()}
	return e
}

func (m *manager) Name() string {
	return m.name
}

// nil on purpose: the listener (and /healthz, /readyz) must come up even when a service behind a route failed
func (m *manager) Dependencies() []common.Service {
	return nil
}

// rebuilds every echo: net/http forbids reusing a server after Shutdown
func (m *manager) Init(ctx context.Context) error {
	for _, s := range m.servers {
		if err := m.rebuildServer(s); err != nil {
			return err
		}
	}
	return nil
}

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

func (m *manager) buildEcho(s *server) error {
	e := m.newEcho()
	e.HTTPErrorHandler = s.errorHandler
	e.Pre(middleware.RemoveTrailingSlash())
	switches, configs, err := s.parseMiddlewareConfigs()
	if err != nil {
		return errors.Wrapf(err, "failed to parse middleware configs for server %s", s.name)
	}
	// order is load-bearing: metrics outside recover so a recovered panic counts as its 500;
	// user middlewares see every request, ahead of logger/info/error, which must stay adjacent
	mw := newMiddleware(s)
	funcs := []echo.MiddlewareFunc{mw.Metrics, mw.Recover}
	// a name with no middleware config entry stays dormant
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

func (m *manager) Add(name string, opts ...ServerOption) error {
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

func (m *manager) Get(name string) (Server, error) {
	s, ok := m.servers[name]
	if !ok {
		return nil, errors.Newf("server %s not found", name)
	}
	return s, nil
}

func (m *manager) List() []Server {
	servers := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	return servers
}

func (m *manager) registerRouter(router api.Router) ([]*handlerGroupConfig, error) {
	data := router.Config()
	if len(data) == 0 {
		return nil, errors.Newf("router %s has empty config", router.Name())
	}
	groups, err := parseRouterConfig(data)
	if err != nil {
		return nil, errors.Wrapf(err, "router %s: failed to parse config", router.Name())
	}
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

// a route declared twice, in any group or router, is a Conflict rather than a silent replacement
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

func (m *manager) RegisterRouters(routers ...api.Router) error {
	for _, r := range routers {
		groups, err := m.registerRouter(r)
		if err != nil {
			return err
		}
		for _, g := range groups {
			serverName := g.Server
			if serverName == "" {
				return errors.Newf("router %s: server name not specified in handler group %q", r.Name(), g.Prefix)
			}

			s, ok := m.servers[serverName]
			if !ok {
				return errors.Newf("server %s not found, please call Add first", serverName)
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

func (m *manager) installHandler(s *server, g *handlerGroupConfig, h *handlerConfig) error {
	// echo concatenates prefix+path literally, so trim to avoid "//health"
	prefix := strings.TrimSuffix(path.Join(s.endpoint.Path, g.Prefix), "/")
	group := s.echo.Group(prefix)

	mwfuncs, err := m.collectMiddlewares(s, h, g)
	if err != nil {
		return err
	}

	key := newHandlerKey(g, h)
	// "/" -> "": with RemoveTrailingSlash both /prefix and /prefix/ match
	routePath := strings.TrimSuffix(h.Path, "/")
	m.log.Infof("register handler %s %s", h.Method, path.Join(prefix, h.Path))

	fn, ok := m.handlerFuncs[key]
	if !ok {
		return nil
	}

	switch h.Method {
	case api.MethodWS:
		group.Add(http.MethodGet, routePath, m.webSocket(s, fn.(wsHandler)), mwfuncs...)
	case api.MethodAny:
		group.Any(routePath, fn.(echo.HandlerFunc), mwfuncs...)
	default:
		group.Add(h.Method, routePath, fn.(echo.HandlerFunc), mwfuncs...)
	}
	return nil
}

// handler refs wrap outermost and shadow same-named group refs; Func runs at registration so a bad config fails startup
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
			// checked before the switch: disabling an unknown middleware is a typo
			return nil, errors.NotImplemented.Newf("middleware %s not found", ref.Name)
		}
		enabled, cfg := resolveMiddleware(ref, groupRefs[ref.Name], serverSwitches, serverConfigs)
		fn, err := mw.Func(enabled, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "middleware %s failed to build for %s %s", ref.Name, h.Method, h.Path)
		}
		if fn == nil {
			continue
		}
		mwfuncs = append(mwfuncs, fn)
	}
	return mwfuncs, nil
}

// most specific first (handler, group, server); switch and config resolve independently; enabled defaults true
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

func (m *manager) Start(ctx context.Context) error {
	for _, s := range m.servers {
		go s.serve()
	}
	return nil
}

// Stop waits for in-flight requests and WebSocket handlers until ctx ends, then closes connections; a handler ignoring its context runs on.
func (m *manager) Stop(ctx context.Context) error {
	var errs []error
	for _, s := range m.servers {
		if err := s.stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Combine(errs...)
}
