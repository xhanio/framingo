package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

func testManager() *manager {
	return newManager(WithLogger(log.New(log.WithLevel(-1))))
}

func testServer(t *testing.T, m *manager, name string) *server {
	t.Helper()
	require.NoError(t, m.Add(name, WithEndpoint("localhost", 8080, "/")))
	s, ok := m.servers[name]
	require.True(t, ok)
	return s
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

func TestRegisterRouter_MethodValidation(t *testing.T) {
	t.Run("accepts valid methods", func(t *testing.T) {
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "ANY"}
		for _, method := range methods {
			m := testManager()
			testServer(t, m, "http")
			router := &mockRouter{
				name: "test",
				config: []byte(`server: http
prefix: /api
handlers:
  - method: ` + method + `
    path: /test
    func: Test`),
				handlers: map[string]echo.HandlerFunc{"Test": okHandler},
			}
			err := m.RegisterRouters(router)
			assert.NoError(t, err, "method %s should be accepted", method)
		}
	})

	t.Run("normalizes lowercase method", func(t *testing.T) {
		m := testManager()
		testServer(t, m, "http")
		router := &mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: any
    path: /test
    func: Test`),
			handlers: map[string]echo.HandlerFunc{"Test": okHandler},
		}
		err := m.RegisterRouters(router)
		assert.NoError(t, err)
	})

	t.Run("rejects invalid method", func(t *testing.T) {
		m := testManager()
		testServer(t, m, "http")
		router := &mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: INVALID
    path: /test
    func: Test`),
			handlers: map[string]echo.HandlerFunc{"Test": okHandler},
		}
		err := m.RegisterRouters(router)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid HTTP method")
	})
}

func TestRequestInfo_ANYKeyFallback(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}
	handler := &api.Handler{Method: api.MethodAny, Path: "/data", Func: "Data"}
	key := api.HandlerKey(group, handler)
	s.handlers[key] = handler
	s.groups[key] = group
	s.echo.Any("/api/data", okHandler)

	// GET request should fall back from http<GET>/api/data to http<ANY>/api/data
	req := httptest.NewRequest("GET", "/api/data", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler, "should resolve via ANY fallback")
	assert.Equal(t, api.MethodAny, info.Handler.Method)
	assert.NotNil(t, info.HandlerGroup)
}

func TestRequestInfo_WildcardKeyMatch(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}
	handler := &api.Handler{Method: "GET", Path: "/*", Func: "CatchAll"}
	key := api.HandlerKey(group, handler)
	s.handlers[key] = handler
	s.groups[key] = group
	s.echo.GET("/api/*", okHandler)

	// concrete path resolves to wildcard pattern after Router().Find()
	req := httptest.NewRequest("GET", "/api/v1/namespaces", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler, "should match wildcard route")
	assert.Equal(t, "/*", info.Handler.Path)
}

func TestRequestInfo_ANYWithWildcardFallback(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}
	handler := &api.Handler{Method: api.MethodAny, Path: "/*", Func: "Proxy"}
	key := api.HandlerKey(group, handler)
	s.handlers[key] = handler
	s.groups[key] = group
	s.echo.Any("/api/*", okHandler)

	// ANY /* should match any method on any sub-path
	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/namespaces"},
		{"POST", "/api/v1/pods"},
		{"DELETE", "/api/resources/123"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		c := s.echo.NewContext(req, httptest.NewRecorder())
		info := s.requestInfo(c)

		assert.NotNil(t, info.Handler, "%s %s should resolve", tt.method, tt.path)
		assert.Equal(t, api.MethodAny, info.Handler.Method)
		assert.Equal(t, "/*", info.Handler.Path)
	}
}

func TestRequestInfo_ExactMethodOverANY(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}

	// Register both an exact GET and an ANY handler for the same path
	getHandler := &api.Handler{Method: "GET", Path: "/data", Func: "GetData"}
	getKey := api.HandlerKey(group, getHandler)
	s.handlers[getKey] = getHandler
	s.groups[getKey] = group

	anyHandler := &api.Handler{Method: api.MethodAny, Path: "/data", Func: "AnyData"}
	anyKey := api.HandlerKey(group, anyHandler)
	s.handlers[anyKey] = anyHandler
	s.groups[anyKey] = group

	s.echo.GET("/api/data", okHandler)
	s.echo.Any("/api/data", okHandler)

	// GET should match the exact handler, not fall back to ANY
	req := httptest.NewRequest("GET", "/api/data", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler)
	assert.Equal(t, "GET", info.Handler.Method, "exact method should take priority over ANY")
}

func TestRequestInfo_WildcardIterationFallback(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	// Store a wildcard handler directly without registering Echo route
	// so Router().Find() won't resolve c.Path() to the pattern
	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}
	handler := &api.Handler{Method: api.MethodAny, Path: "/*", Func: "Proxy"}
	key := api.HandlerKey(group, handler)
	s.handlers[key] = handler
	s.groups[key] = group

	// Without Echo route registration, c.Path() returns the concrete path
	// matchHandler should still find the wildcard via iteration
	req := httptest.NewRequest("GET", "/api/v1/namespaces", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler, "should match via wildcard iteration")
	assert.Equal(t, "/*", info.Handler.Path)
}

func TestRequestInfo_WildcardLongestPrefixWins(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	// Register two wildcard handlers at different depths
	rootGroup := &api.HandlerGroup{Server: "http", Prefix: "/"}
	rootHandler := &api.Handler{Method: api.MethodAny, Path: "/*", Func: "Root"}
	rootKey := api.HandlerKey(rootGroup, rootHandler)
	s.handlers[rootKey] = rootHandler
	s.groups[rootKey] = rootGroup

	apiGroup := &api.HandlerGroup{Server: "http", Prefix: "/api"}
	apiHandler := &api.Handler{Method: api.MethodAny, Path: "/*", Func: "API"}
	apiKey := api.HandlerKey(apiGroup, apiHandler)
	s.handlers[apiKey] = apiHandler
	s.groups[apiKey] = apiGroup

	// /api/v1/namespaces should match /api/* (longer prefix), not /*
	req := httptest.NewRequest("GET", "/api/v1/namespaces", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler)
	assert.Equal(t, "API", info.Handler.Func, "should match longest wildcard prefix")

	// /other/path should match /* (root wildcard)
	req = httptest.NewRequest("GET", "/other/path", nil)
	c = s.echo.NewContext(req, httptest.NewRecorder())
	info = s.requestInfo(c)

	assert.NotNil(t, info.Handler)
	assert.Equal(t, "Root", info.Handler.Func, "should fall back to root wildcard")
}

func TestRequestInfo_WildcardExactMethodOverANY(t *testing.T) {
	m := testManager()
	s := testServer(t, m, "http")

	group := &api.HandlerGroup{Server: "http", Prefix: "/api"}

	// Register both GET /* and ANY /* at the same prefix
	getHandler := &api.Handler{Method: "GET", Path: "/*", Func: "GetWild"}
	getKey := api.HandlerKey(group, getHandler)
	s.handlers[getKey] = getHandler
	s.groups[getKey] = group

	anyHandler := &api.Handler{Method: api.MethodAny, Path: "/*", Func: "AnyWild"}
	anyKey := api.HandlerKey(group, anyHandler)
	s.handlers[anyKey] = anyHandler
	s.groups[anyKey] = group

	// GET should prefer exact method wildcard
	req := httptest.NewRequest("GET", "/api/foo", nil)
	c := s.echo.NewContext(req, httptest.NewRecorder())
	info := s.requestInfo(c)

	assert.NotNil(t, info.Handler)
	assert.Equal(t, "GetWild", info.Handler.Func, "exact method wildcard should win over ANY wildcard")

	// POST should use ANY wildcard
	req = httptest.NewRequest("POST", "/api/foo", nil)
	c = s.echo.NewContext(req, httptest.NewRecorder())
	info = s.requestInfo(c)

	assert.NotNil(t, info.Handler)
	assert.Equal(t, "AnyWild", info.Handler.Func, "POST should fall back to ANY wildcard")
}
