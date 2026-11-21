package wsutil

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
)

func readClose(t *testing.T, conn *websocket.Conn) websocket.CloseError {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	var ce websocket.CloseError
	require.True(t, errors.As(err, &ce), "the client sees a close frame, got %v", err)
	return ce
}

func waitDone(t *testing.T, env *wsTestServer) {
	t.Helper()
	select {
	case <-env.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bridge did not return")
	}
}

// WithFilter writes the messages it keeps, and WithFrame what it makes of them.
func TestBridge_FilterAndFrameShapeTheOutbound(t *testing.T) {
	env := newWSTestServer(t,
		WithFilter(func(msg entity.PubsubMessage) bool { return msg.Kind == "mine" }),
		WithFrame(func(msg entity.PubsubMessage) any { return msg.Payload }),
	)
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()
	<-env.ready

	env.messenger.in <- entity.PubsubMessage{Kind: "theirs", Payload: "not for this session"}
	env.messenger.in <- entity.PubsubMessage{Kind: "mine", Payload: map[string]any{"n": 1}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got map[string]any
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	assert.Equal(t, map[string]any{"n": float64(1)}, got, "the payload alone, the other message kept back")
}

// WithInbound takes the client's frames instead of the bus, and its error ends the
// session with that close.
func TestBridge_InboundHandlesFramesAndItsErrorCloses(t *testing.T) {
	frames := make(chan string, 2)
	env := newWSTestServer(t, WithInbound(func(_ context.Context, data []byte) error {
		frame := strings.TrimSpace(string(data)) // wsjson ends a frame with a newline
		frames <- frame
		if frame == `"bye"` {
			return CloseWith(websocket.StatusPolicyViolation, "unknown frame")
		}
		return nil
	}))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()
	<-env.ready

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, map[string]string{"kind": "ack"}))
	assert.Equal(t, `{"kind":"ack"}`, <-frames)
	require.NoError(t, wsjson.Write(ctx, conn, "bye"))
	ce := readClose(t, conn)
	assert.Equal(t, websocket.StatusPolicyViolation, ce.Code)
	assert.Equal(t, "unknown frame", ce.Reason)
	waitDone(t, env)
	select {
	case msg := <-env.messenger.sent:
		t.Fatalf("a frame reached the bus: %+v", msg)
	default:
	}
}

// WithDone ends the session once its channel closes, with its close: a session another
// displaced is told so, and stops reconnecting.
func TestBridge_DoneEndsTheSessionWithItsClose(t *testing.T) {
	displaced := make(chan struct{})
	env := newWSTestServer(t, WithDone(displaced, CloseWith(websocket.StatusNormalClosure, "displaced")))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()
	<-env.ready

	close(displaced)
	ce := readClose(t, conn)
	assert.Equal(t, websocket.StatusNormalClosure, ce.Code)
	assert.Equal(t, "displaced", ce.Reason)
	waitDone(t, env)
	require.Error(t, env.err, "the close is returned for the server's Close")
}

func TestBridge_DoneWithNoErrorIsANormalClosure(t *testing.T) {
	ended := make(chan struct{})
	env := newWSTestServer(t, WithDone(ended, nil))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()
	<-env.ready

	close(ended)
	assert.Equal(t, websocket.StatusNormalClosure, readClose(t, conn).Code, "a close frame, not a drop")
	waitDone(t, env)
}

// An inbound error that is no CloseWith closes as an internal error, once, and is
// reported, once, for the server to log.
func TestBridge_AnInboundFailureIsClosedOnceAndReported(t *testing.T) {
	broke := errors.Newf("handler broke")
	env := newWSTestServer(t, WithInbound(func(context.Context, []byte) error { return broke }))
	conn := dialWS(t, env.wsURL)
	defer conn.CloseNow()
	<-env.ready

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, "anything"))
	ce := readClose(t, conn)
	assert.Equal(t, websocket.StatusInternalError, ce.Code)
	assert.Equal(t, "handler broke", ce.Reason)
	waitDone(t, env)
	// a nil conn panics if Close closes it again
	assert.ErrorIs(t, Close(context.Background(), nil, env.err), broke, "reported for the log, not sent again")
}
