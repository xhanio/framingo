package server

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
)

func TestRouterGroups_EachDocumentIsAGroup(t *testing.T) {
	text := func(s string) echo.HandlerFunc {
		return func(c echo.Context) error { return c.String(http.StatusOK, s) }
	}
	base, cleanup := startServer(t, &mockRouter{
		name: "users",
		config: []byte(`---
server: http
prefix: /users
handlers:
  - method: GET
    path: /
    func: List
---
server: http
prefix: /user
handlers:
  - method: GET
    path: /
    func: Current
---
`),
		handlers: map[string]any{"List": text("all"), "Current": text("me")},
	})
	defer cleanup()

	code, body := httpDo(t, http.MethodGet, base+"/users")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "all", body)
	code, body = httpDo(t, http.MethodGet, base+"/user")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "me", body)
}

func TestRouterGroups_MiddlewaresStayInTheirGroup(t *testing.T) {
	cap := registerCapture(t, `server: http
prefix: /a
middlewares:
  - capmw:
      marker: a
handlers:
  - method: GET
    path: /on
    func: On
---
server: http
prefix: /b
handlers:
  - method: GET
    path: /off
    func: Off
`)
	require.Len(t, cap.enabled, 1, "only the first group attaches capmw")
	assert.Contains(t, string(cap.configs[0]), "a")
}

func TestRouterGroups_DuplicateRouteConflicts(t *testing.T) {
	route := func(prefix, path string) string {
		return "server: http\nprefix: " + prefix + "\nhandlers:\n  - method: GET\n    path: " + path + "\n    func: Test\n"
	}

	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.RegisterRouters(&mockRouter{
		name:     "twice",
		config:   []byte(route("/api", "/x") + "---\n" + route("/api/x", "/")),
		handlers: map[string]any{"Test": okHandler},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Conflict), "across groups: %v", err)

	m = testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name: "first", config: []byte(route("/api", "/x")), handlers: map[string]any{"Test": okHandler},
	}))
	err = m.RegisterRouters(&mockRouter{
		name: "second", config: []byte(route("/api", "/x")), handlers: map[string]any{"Test": okHandler},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.Conflict), "across routers: %v", err)
}

func TestRouterGroups_NoGroupIsAnError(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.RegisterRouters(&mockRouter{
		name:     "empty",
		config:   []byte("---\n---\n"),
		handlers: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no handler group")
}

func TestRouterGroups_AMistakeInAnyDocumentFailsRegistration(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.RegisterRouters(&mockRouter{
		name:     "users",
		config:   []byte("server: http\nprefix: /a\nhandlers:\n  - {method: GET, path: /, func: A}\n---\nserver: http\nprefix: /b\nhandlers:\n  - {method: GET, path: /, func: B}\n"),
		handlers: map[string]any{"A": okHandler},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "router users, handler group /b")
	assert.Contains(t, err.Error(), "handler function B not found")
}
