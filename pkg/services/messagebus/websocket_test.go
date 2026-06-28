package messagebus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/types/entity"
)

// wsTestServer wires AttachWebSocket up to an httptest server. The handler
// accepts exactly one connection: ready closes after the Messenger is created
// and AttachWebSocket has begun; done closes after AttachWebSocket returns.
type wsTestServer struct {
	mb       Manager
	server   *httptest.Server
	ready    chan struct{}
	done     chan struct{}
	wsURL    string
	attached chan struct{}
}

func newWSTestServer(t *testing.T, name string, opts ...Option) *wsTestServer {
	t.Helper()
	mb := newMessageBus(t, opts...)
	env := &wsTestServer{
		mb:       mb,
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
		attached: make(chan struct{}),
	}
	env.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(env.done)
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer ws.CloseNow()
		messenger, err := mb.NewMessenger(name)
		if err != nil {
			t.Errorf("new messenger failed: %v", err)
			return
		}
		close(env.ready)
		mb.AttachWebSocket(messenger, ws)
		close(env.attached)
	}))
	t.Cleanup(env.server.Close)
	env.wsURL = "ws" + strings.TrimPrefix(env.server.URL, "http")
	return env
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	require.NoError(t, err)
	return conn
}

func TestAttachWebSocket_OutboundDeliversBusMessageToClient(t *testing.T) {
	env := newWSTestServer(t, "ws-out")
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()

	<-env.ready

	ss := &source{}
	env.mb.SendRawMessage(context.Background(), ss, "greet", "hello")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got entity.PubsubMessage
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	assert.Equal(t, "msg_source", got.From)
	assert.Equal(t, "greet", got.Kind)
	assert.Equal(t, "hello", got.Payload)
}

func TestAttachWebSocket_InboundDeliversClientMessageToBus(t *testing.T) {
	env := newWSTestServer(t, "ws-in")
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()

	<-env.ready

	tt := &target{}
	env.mb.Register(tt)

	out := entity.PubsubMessage{Kind: "raw_kind", Payload: "from-client"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, out))

	waitFor(t, 2*time.Second, func() bool { return tt.raw.Load() == 1 })
}

func TestAttachWebSocket_ClientCloseTearsDownSession(t *testing.T) {
	env := newWSTestServer(t, "ws-close")
	conn := dialWS(t, env.wsURL)

	<-env.ready
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "bye"))

	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachWebSocket did not return after client close")
	}
}

func TestAttachWebSocket_PingKeepsConnectionAlive(t *testing.T) {
	env := newWSTestServer(t, "ws-ping-ok", WithPing(50*time.Millisecond, 500*time.Millisecond))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()

	<-env.ready

	// Spawn a client-side read loop so the websocket lib processes the
	// server's ping frames and emits pongs. The server should keep pinging
	// happily without tearing the session down.
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				return
			}
		}
	}()

	// Wait long enough to cover several ping intervals.
	select {
	case <-env.done:
		t.Fatal("AttachWebSocket returned early — pings should not tear down a healthy connection")
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "bye"))
	<-readDone
	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachWebSocket did not return after client close")
	}
}

func TestAttachWebSocket_PingTimeoutClosesSession(t *testing.T) {
	env := newWSTestServer(t, "ws-ping-stall", WithPing(50*time.Millisecond, 150*time.Millisecond))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()

	<-env.ready

	// Never read on the client side. Without a client read loop the
	// websocket lib never processes ping frames or sends pongs, so the
	// server's Ping call times out and the session tears down.
	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("AttachWebSocket did not return after ping timeout")
	}
}

func TestAttachWebSocket_PingDisabledLeavesSessionOpen(t *testing.T) {
	env := newWSTestServer(t, "ws-ping-off", WithPing(0, 0))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()

	<-env.ready

	// Without a client-side reader and without server pings, the session
	// should still stay open — nothing is probing the connection.
	select {
	case <-env.done:
		t.Fatal("AttachWebSocket returned without an explicit close")
	case <-time.After(300 * time.Millisecond):
	}
}
