package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func testManager() *manager {
	return newManager(WithLogger(log.New(log.WithLevel(-1))))
}

type mockRouter struct {
	name     string
	config   []byte
	handlers map[string]any
}

func (r *mockRouter) Name() string                   { return r.name }
func (r *mockRouter) Dependencies() []common.Service { return nil }
func (r *mockRouter) Config() []byte                 { return r.config }
func (r *mockRouter) Handlers() map[string]any       { return r.handlers }

func okHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func freePort(t *testing.T) uint {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return uint(port)
}

func startServer(t *testing.T, routers ...*mockRouter) (baseURL string, cleanup func()) {
	t.Helper()
	m, baseURL := startManager(t, routers...)
	cleanup = func() { require.NoError(t, m.Stop(context.Background())) }
	return baseURL, cleanup
}

// the caller stops it
func startManager(t *testing.T, routers ...*mockRouter) (m *manager, baseURL string) {
	t.Helper()
	port := freePort(t)
	m = testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	for _, r := range routers {
		require.NoError(t, m.RegisterRouters(r))
	}
	require.NoError(t, m.Start(context.Background()))
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	waitServing(t, baseURL)
	return m, baseURL
}

func waitServing(t *testing.T, baseURL string) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
}

func httpDo(t *testing.T, method, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestRegisterRouter_MethodValidation(t *testing.T) {
	t.Run("accepts valid methods", func(t *testing.T) {
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "ANY"}
		for _, method := range methods {
			m := testManager()
			require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
			err := m.RegisterRouters(&mockRouter{
				name: "test",
				config: []byte(`server: http
prefix: /api
handlers:
  - method: ` + method + `
    path: /test
    func: Test`),
				handlers: map[string]any{"Test": okHandler},
			})
			assert.NoError(t, err, "method %s should be accepted", method)
		}
	})

	t.Run("normalizes lowercase method", func(t *testing.T) {
		m := testManager()
		require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
		err := m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: any
    path: /test
    func: Test`),
			handlers: map[string]any{"Test": okHandler},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects invalid method", func(t *testing.T) {
		m := testManager()
		require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
		err := m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: INVALID
    path: /test
    func: Test`),
			handlers: map[string]any{"Test": okHandler},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid HTTP method")
	})
}

func TestHandler_RootPrefix(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /
handlers:
  - method: GET
    path: /health
    func: Health`),
		handlers: map[string]any{"Health": okHandler},
	})
	defer cleanup()

	code, body := httpDo(t, "GET", base+"/health")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body)
}

func TestHandler_ExactKey(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /data
    func: GetData`),
		handlers: map[string]any{"GetData": okHandler},
	})
	defer cleanup()

	code, body := httpDo(t, "GET", base+"/api/data")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body)
}

func TestHandler_ANYFallback(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: ANY
    path: /data
    func: AnyData`),
		handlers: map[string]any{"AnyData": okHandler},
	})
	defer cleanup()

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		code, _ := httpDo(t, method, base+"/api/data")
		assert.Equal(t, http.StatusOK, code, "ANY handler should respond to %s", method)
	}
}

func TestHandler_WildcardPath(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /*
    func: CatchAll`),
		handlers: map[string]any{"CatchAll": okHandler},
	})
	defer cleanup()

	code, body := httpDo(t, "GET", base+"/api/v1/namespaces")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body)
}

func TestHandler_ANYWildcard(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: ANY
    path: /*
    func: Proxy`),
		handlers: map[string]any{"Proxy": okHandler},
	})
	defer cleanup()

	for _, method := range []string{"GET", "POST", "DELETE"} {
		code, _ := httpDo(t, method, base+"/api/v1/namespaces")
		assert.Equal(t, http.StatusOK, code, "%s should match ANY wildcard", method)
	}
}

func TestHandler_WildcardLongestPrefixWins(t *testing.T) {
	appHandler := func(c echo.Context) error { return c.String(http.StatusOK, "app") }
	apiHandler := func(c echo.Context) error { return c.String(http.StatusOK, "api") }

	base, cleanup := startServer(t,
		&mockRouter{
			name: "app-router",
			config: []byte(`server: http
prefix: /app
handlers:
  - method: ANY
    path: /*
    func: App`),
			handlers: map[string]any{"App": appHandler},
		},
		&mockRouter{
			name: "api-router",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: ANY
    path: /*
    func: API`),
			handlers: map[string]any{"API": apiHandler},
		},
	)
	defer cleanup()

	code, body := httpDo(t, "GET", base+"/api/v1/namespaces")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "api", body)

	code, body = httpDo(t, "GET", base+"/app/something")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "app", body)
}

func TestHandler_NoMatch(t *testing.T) {
	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /data
    func: GetData`),
		handlers: map[string]any{"GetData": okHandler},
	})
	defer cleanup()

	code, _ := httpDo(t, "POST", base+"/other")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestWebSocket_Echo(t *testing.T) {
	echoWS := func(c echo.Context, conn *websocket.Conn) error {
		ctx := c.Request().Context()
		for {
			typ, msg, err := conn.Read(ctx)
			if err != nil {
				return nil
			}
			if err := conn.Write(ctx, typ, msg); err != nil {
				return nil
			}
		}
	}

	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /ws
handlers:
  - method: WS
    path: /echo
    func: Echo`),
		handlers: map[string]any{"Echo": echoWS},
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + base[4:] + "/ws/echo" // http:// → ws://
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	msg := []byte("hello websocket")
	err = conn.Write(ctx, websocket.MessageText, msg)
	require.NoError(t, err)

	typ, got, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, websocket.MessageText, typ)
	assert.Equal(t, msg, got)

	msg2 := []byte("second message")
	err = conn.Write(ctx, websocket.MessageText, msg2)
	require.NoError(t, err)

	typ, got, err = conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, websocket.MessageText, typ)
	assert.Equal(t, msg2, got)

	conn.Close(websocket.StatusNormalClosure, "")
}

func TestWebSocket_RegisterRejectsWrongType(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))

	err := m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /ws
handlers:
  - method: WS
    path: /bad
    func: Bad`),
		handlers: map[string]any{"Bad": okHandler},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "declared as WS but signature is not WebSocket")
}

func TestWebSocket_NonWSRequestRejected(t *testing.T) {
	echoWS := func(c echo.Context, conn *websocket.Conn) error {
		return nil
	}

	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /ws
handlers:
  - method: WS
    path: /feed
    func: Feed`),
		handlers: map[string]any{"Feed": echoWS},
	})
	defer cleanup()

	code, _ := httpDo(t, "GET", base+"/ws/feed")
	assert.NotEqual(t, http.StatusSwitchingProtocols, code)
}

func TestWebSocket_HandlerError(t *testing.T) {
	failWS := func(c echo.Context, conn *websocket.Conn) error {
		return fmt.Errorf("something went wrong")
	}

	base, cleanup := startServer(t, &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /ws
handlers:
  - method: WS
    path: /fail
    func: Fail`),
		handlers: map[string]any{"Fail": failWS},
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + base[4:] + "/ws/fail"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)

	_, _, err = conn.Read(ctx)
	assert.Error(t, err)
	var closeErr websocket.CloseError
	assert.ErrorAs(t, err, &closeErr)
	assert.Equal(t, websocket.StatusInternalError, closeErr.Code)
}

type captureMiddleware struct {
	name    string
	enabled []bool
	configs [][]byte
}

func (m *captureMiddleware) Name() string                   { return m.name }
func (m *captureMiddleware) Dependencies() []common.Service { return nil }
func (m *captureMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	m.enabled = append(m.enabled, enabled)
	m.configs = append(m.configs, config)
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
}

type rejectMiddleware struct{}

func (m *rejectMiddleware) Name() string                   { return "rejectmw" }
func (m *rejectMiddleware) Dependencies() []common.Service { return nil }
func (m *rejectMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	if config != nil {
		return nil, fmt.Errorf("rejectmw takes no config")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
}

func TestMiddleware_ConfigForms(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	cap := &captureMiddleware{name: "capmw"}
	require.NoError(t, m.RegisterMiddlewares(cap))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
middlewares:
  - capmw
handlers:
  - method: GET
    path: /bare
    func: A
  - method: GET
    path: /configured
    func: B
    middlewares:
      - capmw:
          key: value
`),
		handlers: map[string]any{"A": okHandler, "B": okHandler},
	}))

	// the handler ref overrides the group's rather than stacking a second run
	require.Len(t, cap.configs, 2)
	assert.Nil(t, cap.configs[0], "bare attachment passes nil config")
	var cfg map[string]string
	require.NoError(t, yaml.Unmarshal(cap.configs[1], &cfg))
	assert.Equal(t, map[string]string{"key": "value"}, cfg)
}

func TestMiddleware_GroupConfig(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	cap := &captureMiddleware{name: "capmw"}
	require.NoError(t, m.RegisterMiddlewares(cap))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
middlewares:
  - capmw:
      scope: group
handlers:
  - method: GET
    path: /a
    func: A
  - method: GET
    path: /b
    func: B
`),
		handlers: map[string]any{"A": okHandler, "B": okHandler},
	}))

	// one Func call per route, so per-route state stays per-route
	require.Len(t, cap.configs, 2)
	for _, raw := range cap.configs {
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(raw, &cfg))
		assert.Equal(t, map[string]string{"scope": "group"}, cfg)
	}
}

func TestMiddleware_ConfigErrorFailsRegistration(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	require.NoError(t, m.RegisterMiddlewares(&rejectMiddleware{}))
	err := m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /a
    func: A
    middlewares:
      - rejectmw:
          unexpected: true
`),
		handlers: map[string]any{"A": okHandler},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejectmw")
}

func TestMiddleware_UnknownNameFailsRegistration(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /a
    func: A
    middlewares:
      - ghost
`),
		handlers: map[string]any{"A": okHandler},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

type stampMiddleware struct{}

func (m *stampMiddleware) Name() string                   { return "stamp" }
func (m *stampMiddleware) Dependencies() []common.Service { return nil }
func (m *stampMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Stamp", "ran")
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}, nil
}

func TestWithMiddlewares_ServerLevel(t *testing.T) {
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http",
		WithEndpoint("127.0.0.1", port, "/"),
		WithMiddlewares(&stampMiddleware{}),
		WithMiddlewareConfigs([]byte("stamp:\n")),
	))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /test
    func: Test`),
		handlers: map[string]any{"Test": okHandler},
	}))
	require.NoError(t, m.Start(context.Background()))
	defer func() { require.NoError(t, m.Stop(context.Background())) }()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)

	t.Run("runs on a matched route", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/test")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "ran", resp.Header.Get("X-Stamp"))
	})

	t.Run("answers a request no route matches, as a preflight needs", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodOptions, baseURL+"/api/test", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "ran", resp.Header.Get("X-Stamp"))
	})

	t.Run("survives a restart rebuild", func(t *testing.T) {
		require.NoError(t, m.Stop(context.Background()))
		require.NoError(t, m.Init(context.Background()))
		require.NoError(t, m.Start(context.Background()))
		require.Eventually(t, func() bool {
			resp, err := http.Get(baseURL + "/api/test")
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return resp.Header.Get("X-Stamp") == "ran"
		}, 2*time.Second, 10*time.Millisecond)
	})
}

type permissionEcho struct{}

func (m *permissionEcho) Name() string                   { return "permecho" }
func (m *permissionEcho) Dependencies() []common.Service { return nil }
func (m *permissionEcho) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req, _ := c.Get(api.ContextKeyRequestInfo).(*api.RequestInfo)
			if req != nil {
				c.Response().Header().Set("X-Permission", req.Permission)
				c.Response().Header().Set("X-Poll", fmt.Sprintf("%t", req.Poll))
			}
			return next(c)
		}
	}, nil
}

func TestRequestInfo_FlattenedRouteMetadata(t *testing.T) {
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	require.NoError(t, m.RegisterMiddlewares(&permissionEcho{}))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /guarded
    func: Guarded
    permission: user.manage
    poll: true
    middlewares:
      - permecho
`),
		handlers: map[string]any{"Guarded": okHandler},
	}))
	require.NoError(t, m.Start(context.Background()))
	defer func() { require.NoError(t, m.Stop(context.Background())) }()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/api/guarded")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.Header.Get("X-Permission") == "user.manage" && resp.Header.Get("X-Poll") == "true"
	}, 2*time.Second, 10*time.Millisecond)
}

func TestMiddlewareDefaults(t *testing.T) {
	newManagerWithDefaults := func(t *testing.T, cap *captureMiddleware) *manager {
		t.Helper()
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", 8080, "/"),
			WithMiddlewareConfigs([]byte("capmw:\n  scope: default\n")),
		))
		require.NoError(t, m.RegisterMiddlewares(cap))
		return m
	}

	t.Run("a bare route attachment receives the server default", func(t *testing.T) {
		cap := &captureMiddleware{name: "capmw"}
		m := newManagerWithDefaults(t, cap)
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /bare
    func: A
    middlewares:
      - capmw`),
			handlers: map[string]any{"A": okHandler},
		}))
		require.Len(t, cap.configs, 1)
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(cap.configs[0], &cfg))
		assert.Equal(t, map[string]string{"scope": "default"}, cfg)
	})

	t.Run("a route's own config beats the server default", func(t *testing.T) {
		cap := &captureMiddleware{name: "capmw"}
		m := newManagerWithDefaults(t, cap)
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /own
    func: A
    middlewares:
      - capmw:
          scope: route`),
			handlers: map[string]any{"A": okHandler},
		}))
		require.Len(t, cap.configs, 1)
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(cap.configs[0], &cfg))
		assert.Equal(t, map[string]string{"scope": "route"}, cfg)
	})

	t.Run("a server-level middleware receives its default", func(t *testing.T) {
		cap := &captureMiddleware{name: "capmw"}
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", 8080, "/"),
			WithMiddlewares(cap),
			WithMiddlewareConfigs([]byte("capmw:\n  scope: server\n")),
		))
		require.NotEmpty(t, cap.configs)
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(cap.configs[len(cap.configs)-1], &cfg))
		assert.Equal(t, map[string]string{"scope": "server"}, cfg)
	})

	t.Run("a name mapped to null carries no config", func(t *testing.T) {
		cap := &captureMiddleware{name: "capmw"}
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", 8080, "/"),
			WithMiddlewares(cap),
			WithMiddlewareConfigs([]byte("capmw:\n")),
		))
		require.NotEmpty(t, cap.configs)
		assert.Nil(t, cap.configs[len(cap.configs)-1])
	})

	t.Run("invalid defaults fail Add", func(t *testing.T) {
		m := testManager()
		err := m.Add("http",
			WithEndpoint("127.0.0.1", 8080, "/"),
			WithMiddlewareConfigs([]byte("not: [valid")),
		)
		assert.Error(t, err)
	})
}

func TestMiddleware_ConfigPrecedence(t *testing.T) {
	cap := &captureMiddleware{name: "capmw"}
	m := testManager()
	require.NoError(t, m.Add("http",
		WithEndpoint("127.0.0.1", 8080, "/"),
		WithMiddlewareConfigs([]byte("capmw:\n  scope: server\n")),
	))
	require.NoError(t, m.RegisterMiddlewares(cap))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
middlewares:
  - capmw:
      scope: group
handlers:
  - method: GET
    path: /bare-handler
    func: A
    middlewares:
      - capmw
  - method: GET
    path: /group-only
    func: B
  - method: GET
    path: /own
    func: C
    middlewares:
      - capmw:
          scope: handler
`),
		handlers: map[string]any{"A": okHandler, "B": okHandler, "C": okHandler},
	}))

	require.Len(t, cap.configs, 3)
	scopes := make([]string, 0, 3)
	for _, raw := range cap.configs {
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(raw, &cfg))
		scopes = append(scopes, cfg["scope"])
	}
	assert.Equal(t, []string{"group", "group", "handler"}, scopes)
}

func TestMiddleware_DeclineAttachment(t *testing.T) {
	t.Run("a server-level middleware may decline attachment", func(t *testing.T) {
		port := freePort(t)
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", port, "/"),
			WithMiddlewares(&decliningMiddleware{}),
			WithMiddlewareConfigs([]byte("declining:\n")),
		))
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /test
    func: Test`),
			handlers: map[string]any{"Test": okHandler},
		}))
		require.NoError(t, m.Start(context.Background()))
		defer func() { require.NoError(t, m.Stop(context.Background())) }()
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		require.Eventually(t, func() bool {
			resp, err := http.Get(baseURL + "/api/test")
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}, 2*time.Second, 10*time.Millisecond)
	})
}

type decliningMiddleware struct{}

func (m *decliningMiddleware) Name() string                   { return "declining" }
func (m *decliningMiddleware) Dependencies() []common.Service { return nil }
func (m *decliningMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return nil, nil
}

type panicMiddleware struct{}

func (m *panicMiddleware) Name() string                   { return "panicky" }
func (m *panicMiddleware) Dependencies() []common.Service { return nil }
func (m *panicMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			panic("server-level middleware panic")
		}
	}, nil
}

type witnessMiddleware struct {
	saw []string
}

func (m *witnessMiddleware) Name() string                   { return "witness" }
func (m *witnessMiddleware) Dependencies() []common.Service { return nil }
func (m *witnessMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			m.saw = append(m.saw, c.Request().Method)
			return next(c)
		}
	}, nil
}

func TestChain_Order(t *testing.T) {
	t.Run("recover wraps server-level middlewares", func(t *testing.T) {
		port := freePort(t)
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", port, "/"),
			WithMiddlewares(&panicMiddleware{}),
			WithMiddlewareConfigs([]byte("panicky:\n")),
		))
		require.NoError(t, m.Start(context.Background()))
		defer func() { require.NoError(t, m.Stop(context.Background())) }()
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

		require.Eventually(t, func() bool {
			resp, err := http.Get(baseURL + "/anything")
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return resp.StatusCode == http.StatusInternalServerError && len(body) > 0
		}, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("server-level middlewares run in WithMiddlewares order", func(t *testing.T) {
		port := freePort(t)
		m := testManager()
		witness := &witnessMiddleware{}
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", port, "/"),
			WithMiddlewares(&optionsShortCircuitMiddleware{}, witness),
			WithMiddlewareConfigs([]byte("optshort:\nwitness:\n")),
		))
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /test
    func: Test`),
			handlers: map[string]any{"Test": okHandler},
		}))
		require.NoError(t, m.Start(context.Background()))
		defer func() { require.NoError(t, m.Stop(context.Background())) }()
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		require.Eventually(t, func() bool {
			resp, err := http.Get(baseURL + "/api/test")
			if err != nil {
				return false
			}
			resp.Body.Close()
			return true
		}, 2*time.Second, 10*time.Millisecond)

		req, err := http.NewRequest(http.MethodOptions, baseURL+"/api/test", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode)

		assert.Contains(t, witness.saw, http.MethodGet)
		assert.NotContains(t, witness.saw, http.MethodOptions)
	})
}

type optionsShortCircuitMiddleware struct{}

func (m *optionsShortCircuitMiddleware) Name() string                   { return "optshort" }
func (m *optionsShortCircuitMiddleware) Dependencies() []common.Service { return nil }
func (m *optionsShortCircuitMiddleware) Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	if !enabled {
		return nil, nil
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}, nil
}

func TestMiddleware_DeclineAtRoute(t *testing.T) {
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	require.NoError(t, m.RegisterMiddlewares(&decliningMiddleware{}))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
middlewares:
  - declining
handlers:
  - method: GET
    path: /test
    func: Test
    middlewares:
      - declining`),
		handlers: map[string]any{"Test": okHandler},
	}))
	require.NoError(t, m.Start(context.Background()))
	defer func() { require.NoError(t, m.Stop(context.Background())) }()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/api/test")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode == http.StatusOK && string(body) == "ok"
	}, 2*time.Second, 10*time.Millisecond)
}

func TestMiddleware_NullAndEmptyBlocks(t *testing.T) {
	t.Run("a null handler entry inherits the group's block", func(t *testing.T) {
		cap := &captureMiddleware{name: "capmw"}
		m := testManager()
		require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
		require.NoError(t, m.RegisterMiddlewares(cap))
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
middlewares:
  - capmw:
      scope: group
handlers:
  - method: GET
    path: /a
    func: A
    middlewares:
      - capmw:
`),
			handlers: map[string]any{"A": okHandler},
		}))
		require.Len(t, cap.configs, 1)
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(cap.configs[0], &cfg))
		assert.Equal(t, map[string]string{"scope": "group"}, cfg)
	})

	t.Run("an empty block shadows instead of inheriting", func(t *testing.T) {
		// `- name: {}` opts out of inherited config without detaching
		cap := &captureMiddleware{name: "capmw"}
		m := testManager()
		require.NoError(t, m.Add("http",
			WithEndpoint("127.0.0.1", 8080, "/"),
			WithMiddlewareConfigs([]byte("capmw:\n  scope: server\n")),
		))
		require.NoError(t, m.RegisterMiddlewares(cap))
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /a
    func: A
    middlewares:
      - capmw: {}
`),
			handlers: map[string]any{"A": okHandler},
		}))
		require.Len(t, cap.configs, 1)
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(cap.configs[0], &cfg))
		assert.Empty(t, cfg)
	})
}

func TestMiddlewareConfigs_SurviveRestartRebuild(t *testing.T) {
	cap := &captureMiddleware{name: "capmw"}
	m := testManager()
	require.NoError(t, m.Add("http",
		WithEndpoint("127.0.0.1", 8080, "/"),
		WithMiddlewares(cap),
		WithMiddlewareConfigs([]byte("capmw:\n  scope: server\n")),
	))
	require.NoError(t, m.RegisterMiddlewares(cap))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /a
    func: A
    middlewares:
      - capmw`),
		handlers: map[string]any{"A": okHandler},
	}))
	before := len(cap.configs)
	require.NoError(t, m.Init(context.Background()))
	require.Greater(t, len(cap.configs), before)
	for i, raw := range cap.configs {
		var cfg map[string]string
		require.NoError(t, yaml.Unmarshal(raw, &cfg), "config %d", i)
		assert.Equal(t, map[string]string{"scope": "server"}, cfg, "config %d", i)
	}
}

func TestAdd_RejectsDuplicateName(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.Add("http", WithEndpoint("127.0.0.1", 8081, "/"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Conflict), "expected Conflict, got %v", err)
	assert.EqualValues(t, 8080, m.servers["http"].endpoint.Port, "the first instance must survive")
}

func TestStopClosesWhatStillServesOnceItsContextEnds(t *testing.T) {
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "slow",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /slow
    func: Slow`),
		handlers: map[string]any{"Slow": func(c echo.Context) error {
			close(entered)
			<-release
			return c.String(http.StatusOK, "late")
		}},
	}))
	require.NoError(t, m.Start(context.Background()))
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
	answered := make(chan error, 1)
	go func() {
		resp, err := http.Get(baseURL + "/api/slow")
		if err == nil {
			resp.Body.Close()
		}
		answered <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded)
	select {
	case err := <-answered:
		assert.Error(t, err, "the request still going got an answer instead of a closed connection")
	case <-time.After(3 * time.Second):
		t.Fatal("the connection still going was left open")
	}
}

// https without a private key is refused when the server is added, not by every handshake.
func TestAddRefusesHTTPSWithoutAKey(t *testing.T) {
	ca, err := certutil.New()
	require.NoError(t, err)
	signed, err := ca.SignServer(&certutil.ServerRequest{CommonName: "srv.local"})
	require.NoError(t, err)
	certOnly, err := certutil.NewCertBundleWithoutKey(signed.CertPEM())
	require.NoError(t, err)

	m := testManager()
	err = m.Add("https", WithEndpoint("127.0.0.1", 8443, "/"), WithTLS(certOnly))
	assert.True(t, err != nil && errors.Is(err, errors.InvalidArgument), "a certificate without its key: got %v", err)
	require.NoError(t, m.Add("https", WithEndpoint("127.0.0.1", 8443, "/"), WithTLS(signed)), "with its key")
}

// CheckRouter is registration without a server: what would fail startup fails here.
func TestCheckRouter(t *testing.T) {
	config := []byte("server: http\nprefix: /orders\nhandlers:\n  - method: get\n    path: /:id\n    func: Get\n    permission: order_read\n")
	routes, err := CheckRouter(&mockRouter{name: "orders", config: config, handlers: map[string]any{"Get": okHandler}})
	require.NoError(t, err)
	assert.Equal(t, []Route{{Server: "http", Prefix: "/orders", Method: "GET", Path: "/orders/:id", Func: "Get", Permission: "order_read"}}, routes)

	_, err = CheckRouter(&mockRouter{name: "orders", config: config, handlers: map[string]any{"List": okHandler}})
	assert.True(t, err != nil && errors.Is(err, errors.NotImplemented), "a func Handlers lacks: got %v", err)

	_, err = CheckRouter(&mockRouter{name: "orders", config: config, handlers: map[string]any{"Get": func() {}}})
	assert.ErrorContains(t, err, "unsupported signature")

	noServer := []byte("prefix: /orders\nhandlers:\n  - method: GET\n    path: /\n    func: Get\n")
	_, err = CheckRouter(&mockRouter{name: "orders", config: noServer, handlers: map[string]any{"Get": okHandler}})
	assert.ErrorContains(t, err, "server name not specified")

	twice := append(append([]byte{}, config...), []byte("  - method: GET\n    path: /:id\n    func: Get\n")...)
	_, err = CheckRouter(&mockRouter{name: "orders", config: twice, handlers: map[string]any{"Get": okHandler}})
	assert.True(t, err != nil && errors.Is(err, errors.Conflict), "a route declared twice: got %v", err)
}
