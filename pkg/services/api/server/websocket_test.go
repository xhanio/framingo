package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wsRouter(fn func(echo.Context, *websocket.Conn) error) *mockRouter {
	return &mockRouter{
		name: "test",
		config: []byte(`server: http
prefix: /ws
handlers:
  - method: WS
    path: /conn
    func: Conn`),
		handlers: map[string]any{"Conn": fn},
	}
}

func dialConn(t *testing.T, baseURL string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(baseURL, "http")+"/ws/conn", nil)
	require.NoError(t, err)
	return conn
}

func TestWebSocket_StopClosesConnectionsGoingAway(t *testing.T) {
	entered := make(chan struct{})
	returned := make(chan struct{})
	m, base := startManager(t, wsRouter(func(c echo.Context, conn *websocket.Conn) error {
		defer close(returned)
		close(entered)
		_, _, err := conn.Read(context.Background())
		return err
	}))
	conn := dialConn(t, base)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()
	<-entered

	stopped := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopped <- m.Stop(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	assert.Equal(t, websocket.StatusGoingAway, websocket.CloseStatus(err), "the client is told the server is going away, got %v", err)
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
	select {
	case <-returned:
	default:
		t.Fatal("Stop returned before the handler did")
	}
}

func TestWebSocket_StopGivesUpOnAHandlerThatIgnoresItsConnection(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	m, base := startManager(t, wsRouter(func(c echo.Context, conn *websocket.Conn) error {
		close(entered)
		<-release
		return nil
	}))
	defer close(release)
	conn := dialConn(t, base)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()
	// reading lets the client answer the server's close frame
	//nolint:errcheck // the read ends with the close; nothing to check
	go conn.Read(context.Background())
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := m.Stop(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "websocket handlers still running")
	assert.Less(t, time.Since(start), 2*time.Second, "Stop gives up once its context ends")
}

func TestWebSocket_RestartServesAgain(t *testing.T) {
	m, base := startManager(t, wsRouter(func(c echo.Context, conn *websocket.Conn) error {
		ctx := c.Request().Context()
		for {
			typ, msg, err := conn.Read(ctx)
			if err != nil {
				return err
			}
			if err := conn.Write(ctx, typ, msg); err != nil {
				return err
			}
		}
	}))
	require.NoError(t, m.Stop(context.Background()))
	require.NoError(t, m.Init(context.Background()))
	require.NoError(t, m.Start(context.Background()))
	waitServing(t, base)
	defer func() { require.NoError(t, m.Stop(context.Background())) }()

	conn := dialConn(t, base)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte("again")))
	_, got, err := conn.Read(ctx)
	require.NoError(t, err, "the connection is served, not refused")
	assert.Equal(t, "again", string(got))
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
}

func TestWebSocket_UpgradeAfterStopIsRefused(t *testing.T) {
	m, _ := startManager(t, wsRouter(func(c echo.Context, conn *websocket.Conn) error { return nil }))
	require.NoError(t, m.Stop(context.Background()))
	_, ok := m.servers["http"].trackWebSocket()
	assert.False(t, ok, "an upgrade after the stop began is not served")
}
