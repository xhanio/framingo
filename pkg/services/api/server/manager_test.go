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
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func testManager() *manager {
	return newManager(WithLogger(log.New(log.WithLevel(-1))))
}

// mockRouter implements api.Router for testing
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

// freePort returns an available TCP port.
func freePort(t *testing.T) uint {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return uint(port)
}

// startServer sets up a manager with routers, starts the server, and returns
// the base URL and a cleanup function.
func startServer(t *testing.T, routers ...*mockRouter) (baseURL string, cleanup func()) {
	t.Helper()
	port := freePort(t)
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", port, "/")))
	for _, r := range routers {
		require.NoError(t, m.RegisterRouters(r))
	}
	require.NoError(t, m.Start(context.Background()))
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond)
	cleanup = func() { require.NoError(t, m.Stop(true)) }
	return
}

// httpDo makes an HTTP request and returns status code and body.
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

// ============================================================================
// WebSocket Tests
// ============================================================================

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

	// Connect via WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + base[4:] + "/ws/echo" // http:// → ws://
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	// Send and receive a message
	msg := []byte("hello websocket")
	err = conn.Write(ctx, websocket.MessageText, msg)
	require.NoError(t, err)

	typ, got, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, websocket.MessageText, typ)
	assert.Equal(t, msg, got)

	// Send another message
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

	// WS method with an echo.HandlerFunc should fail
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

	// Plain HTTP GET to a WS endpoint should not succeed as WebSocket
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

	// Server should close with StatusInternalError
	_, _, err = conn.Read(ctx)
	assert.Error(t, err)
	var closeErr websocket.CloseError
	assert.ErrorAs(t, err, &closeErr)
	assert.Equal(t, websocket.StatusInternalError, closeErr.Code)
}

// captureMiddleware records the config bytes of every Func call, so a test can
// see how many attachments were minted and what each one received.
type captureMiddleware struct {
	name    string
	configs [][]byte
}

func (m *captureMiddleware) Name() string                   { return m.name }
func (m *captureMiddleware) Dependencies() []common.Service { return nil }
func (m *captureMiddleware) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
	m.configs = append(m.configs, config)
	return func(next echo.HandlerFunc) echo.HandlerFunc { return next }, nil
}

// rejectMiddleware refuses any config, the shape of a config-free middleware
// handed a block it does not understand.
type rejectMiddleware struct{}

func (m *rejectMiddleware) Name() string                   { return "rejectmw" }
func (m *rejectMiddleware) Dependencies() []common.Service { return nil }
func (m *rejectMiddleware) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
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

	// One Func call per route: /bare from the group ref, /configured from the
	// handler ref. The handler's configured ref overrides the group's bare one
	// rather than stacking a second run.
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

	// A group-level config reaches every route of the group, one Func call
	// each, so per-route state stays per-route.
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

// stampMiddleware is shaped like CORS: attached at server level from code, it
// stamps every response and answers OPTIONS itself, before routing.
type stampMiddleware struct{}

func (m *stampMiddleware) Name() string                   { return "stamp" }
func (m *stampMiddleware) Dependencies() []common.Service { return nil }
func (m *stampMiddleware) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
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
	defer func() { require.NoError(t, m.Stop(true)) }()
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
		require.NoError(t, m.Stop(true))
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

// permissionEcho answers with the flattened route metadata it sees at request
// time, proving the schema never has to leave the server package.
type permissionEcho struct{}

func (m *permissionEcho) Name() string                   { return "permecho" }
func (m *permissionEcho) Dependencies() []common.Service { return nil }
func (m *permissionEcho) Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error) {
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
	defer func() { require.NoError(t, m.Stop(true)) }()
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
