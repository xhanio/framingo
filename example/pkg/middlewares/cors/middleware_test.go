package cors

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/types/common"
)

func TestName(t *testing.T) {
	assert.Equal(t, "cors", New().Name())
}

func TestFunc(t *testing.T) {
	t.Run("disabled, it declines attachment", func(t *testing.T) {
		fn, err := New().Func(false, nil)
		require.NoError(t, err)
		assert.Nil(t, fn)
	})

	t.Run("enabled with no policy, it attaches the permissive defaults", func(t *testing.T) {
		fn, err := New().Func(true, nil)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("a policy block attaches", func(t *testing.T) {
		fn, err := New().Func(true, []byte("allow_origins:\n  - https://app.example.com\n"))
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("an unknown policy field fails startup", func(t *testing.T) {
		_, err := New().Func(true, []byte("allowed_origins:\n  - https://a.example\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cors")
	})

	t.Run("credentials against a wildcard origin fails startup", func(t *testing.T) {
		_, err := New().Func(true, []byte("allow_credentials: true\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allow_origins")
	})
}

// stubRouter is the minimal fapi.Router the integration test needs.
type stubRouter struct{}

func (r *stubRouter) Name() string                   { return "corstest" }
func (r *stubRouter) Dependencies() []common.Service { return nil }
func (r *stubRouter) Config() []byte {
	return []byte("server: http\nprefix: /api\nhandlers:\n  - method: GET\n    path: /test\n    func: Test\n")
}
func (r *stubRouter) Handlers() map[string]any {
	return map[string]any{"Test": echo.HandlerFunc(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})}
}

// The wiring the server component performs - WithMiddlewares(New()) plus a
// cors block in the middleware configs - answers preflight on a live server.
func TestPreflightThroughServer(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := uint(l.Addr().(*net.TCPAddr).Port)
	require.NoError(t, l.Close())

	m := server.New()
	require.NoError(t, m.Add("http",
		server.WithEndpoint("127.0.0.1", port, "/"),
		server.WithMiddlewares(New()),
		server.WithMiddlewareConfigs([]byte("cors: true\n")),
	))
	require.NoError(t, m.RegisterRouters(&stubRouter{}))
	require.NoError(t, m.Start(context.Background()))
	defer func() { require.NoError(t, m.Stop(true)) }()

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
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodGet)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get(echo.HeaderAccessControlAllowOrigin))
}
