package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The enabled flag is resolved by the framework and passed to every Func:
// booleans under a middleware's name are switches, most specific first;
// config resolves independently of the switch.

func registerCapture(t *testing.T, routerYAML string, opts ...ServerOption) *captureMiddleware {
	t.Helper()
	m := testManager()
	require.NoError(t, m.Add("http", append([]ServerOption{WithEndpoint("127.0.0.1", 8080, "/")}, opts...)...))
	cap := &captureMiddleware{name: "capmw"}
	require.NoError(t, m.RegisterMiddlewares(cap))
	require.NoError(t, m.RegisterRouters(&mockRouter{
		name:     "test",
		config:   []byte(routerYAML),
		handlers: map[string]any{"On": okHandler, "Off": okHandler},
	}))
	return cap
}

func TestEnabled_HandlerFalseDisables(t *testing.T) {
	cap := registerCapture(t, `server: http
prefix: /api
middlewares:
  - capmw:
      marker: group
handlers:
  - method: GET
    path: /on
    func: On
  - method: GET
    path: /off
    func: Off
    middlewares:
      - capmw: false
`)
	require.Equal(t, []bool{true, false}, cap.enabled)
	// The switch and the config resolve independently: the disabled
	// attachment still sees the group's block.
	assert.Contains(t, string(cap.configs[0]), "group")
	assert.Contains(t, string(cap.configs[1]), "group")
}

func TestEnabled_TrueOverridesGroupFalse(t *testing.T) {
	cap := registerCapture(t, `server: http
prefix: /api
middlewares:
  - capmw: false
handlers:
  - method: GET
    path: /on
    func: On
    middlewares:
      - capmw: true
  - method: GET
    path: /off
    func: Off
`)
	// Most specific boolean wins in both directions.
	require.Equal(t, []bool{true, false}, cap.enabled)
}

func TestEnabled_ServerMappingSwitchAndConfig(t *testing.T) {
	t.Run("a false in the server mapping switches bare refs off", func(t *testing.T) {
		cap := registerCapture(t, `server: http
prefix: /api
handlers:
  - method: GET
    path: /on
    func: On
    middlewares:
      - capmw
`, WithMiddlewareConfigs([]byte("capmw: false\n")))
		require.Equal(t, []bool{false}, cap.enabled)
	})

	t.Run("a block in the server mapping configures bare refs", func(t *testing.T) {
		cap := registerCapture(t, `server: http
prefix: /api
handlers:
  - method: GET
    path: /on
    func: On
    middlewares:
      - capmw
`, WithMiddlewareConfigs([]byte("capmw:\n  marker: server\n")))
		require.Equal(t, []bool{true}, cap.enabled)
		assert.Contains(t, string(cap.configs[0]), "server")
	})
}

func TestEnabled_QuotedFalseIsConfig(t *testing.T) {
	cap := registerCapture(t, `server: http
prefix: /api
handlers:
  - method: GET
    path: /on
    func: On
    middlewares:
      - capmw: "false"
`)
	// The quoted string is config, not a switch - the escape hatch for a
	// middleware whose config genuinely looks boolean.
	require.Equal(t, []bool{true}, cap.enabled)
	assert.Contains(t, string(cap.configs[0]), "false")
}

func TestEnabled_UnknownNameStillFails(t *testing.T) {
	m := testManager()
	require.NoError(t, m.Add("http", WithEndpoint("127.0.0.1", 8080, "/")))
	err := m.RegisterRouters(&mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /h
    func: On
    middlewares:
      - ghost: false
`),
		handlers: map[string]any{"On": okHandler},
	})
	// Switching off a middleware that does not exist is a typo, not a no-op.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

// Server-level middlewares are declared by WithMiddlewares and activated by
// the server's middleware configs.
func TestEnabled_ServerLevelActivation(t *testing.T) {
	boot := func(t *testing.T, configs []byte) *witnessMiddleware {
		t.Helper()
		port := freePort(t)
		m := testManager()
		witness := &witnessMiddleware{}
		opts := []ServerOption{WithEndpoint("127.0.0.1", port, "/"), WithMiddlewares(witness)}
		if configs != nil {
			opts = append(opts, WithMiddlewareConfigs(configs))
		}
		require.NoError(t, m.Add("http", opts...))
		require.NoError(t, m.RegisterRouters(&mockRouter{
			name: "test",
			config: []byte(`server: http
prefix: /api
handlers:
  - method: GET
    path: /test
    func: On`),
			handlers: map[string]any{"On": okHandler},
		}))
		require.NoError(t, m.Start(context.Background()))
		t.Cleanup(func() { require.NoError(t, m.Stop(true)) })
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		require.Eventually(t, func() bool {
			resp, err := http.Get(baseURL + "/api/test")
			if err != nil {
				return false
			}
			resp.Body.Close()
			return true
		}, 2*time.Second, 10*time.Millisecond)
		return witness
	}

	t.Run("no entry leaves it dormant", func(t *testing.T) {
		witness := boot(t, nil)
		assert.Empty(t, witness.saw)
	})

	t.Run("a null entry activates it", func(t *testing.T) {
		witness := boot(t, []byte("witness:\n"))
		assert.Contains(t, witness.saw, http.MethodGet)
	})

	t.Run("false keeps it off", func(t *testing.T) {
		witness := boot(t, []byte("witness: false\n"))
		assert.Empty(t, witness.saw)
	})

	t.Run("a block activates and configures", func(t *testing.T) {
		witness := boot(t, []byte("witness:\n  anything: goes\n"))
		assert.Contains(t, witness.saw, http.MethodGet)
	})
}
