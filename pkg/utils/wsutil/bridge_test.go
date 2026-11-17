package wsutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
)

var _ model.Messenger = (*fakeMessenger)(nil)

// fakeMessenger stands in for a bus Messenger: what a test puts on in is what
// the bridge writes to the client, and what the bridge sends lands on sent.
type fakeMessenger struct {
	in     chan entity.PubsubMessage
	sent   chan entity.PubsubMessage
	once   sync.Once
	closed chan struct{}
}

func newFakeMessenger() *fakeMessenger {
	return &fakeMessenger{
		in:     make(chan entity.PubsubMessage, 1),
		sent:   make(chan entity.PubsubMessage, 1),
		closed: make(chan struct{}),
	}
}

func (f *fakeMessenger) Name() string                    { return "fake" }
func (f *fakeMessenger) Ch() <-chan entity.PubsubMessage { return f.in }
func (f *fakeMessenger) Close()                          { f.once.Do(func() { close(f.closed) }) }

func (f *fakeMessenger) Send(ctx context.Context, kind string, payload any) error {
	select {
	case f.sent <- entity.PubsubMessage{From: f.Name(), Kind: kind, Payload: payload}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// wsTestServer wires Bridge up to an httptest server. The handler
// accepts exactly one connection: ready closes once it is accepted and the
// bridge is about to begin; done closes after Bridge returns.
type wsTestServer struct {
	messenger *fakeMessenger
	ready     chan struct{}
	done      chan struct{}
	wsURL     string
}

func newWSTestServer(t *testing.T, opts ...Option) *wsTestServer {
	t.Helper()
	env := &wsTestServer{
		messenger: newFakeMessenger(),
		ready:     make(chan struct{}),
		done:      make(chan struct{}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(env.done)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		//nolint:errcheck // nothing to do with a failure to close here
		defer conn.CloseNow()
		close(env.ready)
		Bridge(conn, env.messenger, opts...)
	}))
	t.Cleanup(server.Close)
	env.wsURL = "ws" + strings.TrimPrefix(server.URL, "http")
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

func TestBridge_OutboundDeliversMessageToClient(t *testing.T) {
	env := newWSTestServer(t)
	conn := dialWS(t, env.wsURL)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()

	<-env.ready
	env.messenger.in <- entity.PubsubMessage{From: "msg_source", Kind: "greet", Payload: "hello"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got entity.PubsubMessage
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	assert.Equal(t, "msg_source", got.From)
	assert.Equal(t, "greet", got.Kind)
	assert.Equal(t, "hello", got.Payload)
}

func TestBridge_InboundSendsClientMessageThroughMessenger(t *testing.T) {
	env := newWSTestServer(t)
	conn := dialWS(t, env.wsURL)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()

	<-env.ready
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, entity.PubsubMessage{Kind: "raw_kind", Payload: "from-client"}))

	select {
	case got := <-env.messenger.sent:
		assert.Equal(t, "raw_kind", got.Kind)
		assert.Equal(t, "from-client", got.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("the client's message was not sent through the messenger")
	}
}

func TestBridge_ClientCloseTearsDownSession(t *testing.T) {
	env := newWSTestServer(t)
	conn := dialWS(t, env.wsURL)

	<-env.ready
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "bye"))

	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bridge did not return after client close")
	}
	select {
	case <-env.messenger.closed:
	default:
		t.Fatal("Bridge returned without closing the messenger")
	}
}

func TestBridge_PingKeepsConnectionAlive(t *testing.T) {
	env := newWSTestServer(t, WithPing(50*time.Millisecond, 500*time.Millisecond))
	conn := dialWS(t, env.wsURL)
	//nolint:errcheck // nothing to do with a failure to close here
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
		t.Fatal("Bridge returned early — pings should not tear down a healthy connection")
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "bye"))
	<-readDone
	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bridge did not return after client close")
	}
}

func TestBridge_PingTimeoutClosesSession(t *testing.T) {
	env := newWSTestServer(t, WithPing(50*time.Millisecond, 150*time.Millisecond))
	conn := dialWS(t, env.wsURL)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()

	<-env.ready

	// Never read on the client side. Without a client read loop the
	// websocket lib never processes ping frames or sends pongs, so the
	// server's Ping call times out and the session tears down.
	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bridge did not return after ping timeout")
	}
}

func TestBridge_PingDisabledLeavesSessionOpen(t *testing.T) {
	env := newWSTestServer(t, WithPing(0, 0))
	conn := dialWS(t, env.wsURL)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()

	<-env.ready

	// Without a client-side reader and without server pings, the session
	// should still stay open — nothing is probing the connection.
	select {
	case <-env.done:
		t.Fatal("Bridge returned without an explicit close")
	case <-time.After(300 * time.Millisecond):
	}
}
