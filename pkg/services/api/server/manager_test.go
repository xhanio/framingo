package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	handlers map[string]echo.HandlerFunc
}

func (r *mockRouter) Name() string                          { return r.name }
func (r *mockRouter) Dependencies() []common.Service        { return nil }
func (r *mockRouter) Config() []byte                        { return r.config }
func (r *mockRouter) Handlers() map[string]echo.HandlerFunc { return r.handlers }

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
				handlers: map[string]echo.HandlerFunc{"Test": okHandler},
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
			handlers: map[string]echo.HandlerFunc{"Test": okHandler},
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
			handlers: map[string]echo.HandlerFunc{"Test": okHandler},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid HTTP method")
	})
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
		handlers: map[string]echo.HandlerFunc{"GetData": okHandler},
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
		handlers: map[string]echo.HandlerFunc{"AnyData": okHandler},
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
		handlers: map[string]echo.HandlerFunc{"CatchAll": okHandler},
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
		handlers: map[string]echo.HandlerFunc{"Proxy": okHandler},
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
			handlers: map[string]echo.HandlerFunc{"App": appHandler},
		},
		&mockRouter{
			name: "api-router",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: ANY
    path: /*
    func: API`),
			handlers: map[string]echo.HandlerFunc{"API": apiHandler},
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
		handlers: map[string]echo.HandlerFunc{"GetData": okHandler},
	})
	defer cleanup()

	code, _ := httpDo(t, "POST", base+"/other")
	assert.Equal(t, http.StatusNotFound, code)
}
