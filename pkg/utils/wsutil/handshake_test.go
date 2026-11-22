package wsutil

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
)

type registration struct {
	SessionID string `json:"session_id"`
}

// handshakeServer runs ReadFirst as a handler would, the server closing by what it returned.
func handshakeServer(t *testing.T, timeout time.Duration) (string, <-chan error, <-chan registration) {
	t.Helper()
	errs := make(chan error, 1)
	regs := make(chan registration, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer conn.CloseNow()
		reg, err := ReadFirst[registration](context.Background(), conn, timeout, "registration required")
		if err == nil {
			regs <- reg
		}
		errs <- err
		_ = Close(context.Background(), conn, err)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http"), errs, regs
}

func TestReadFirst_DecodesTheFirstFrame(t *testing.T) {
	url, errs, regs := handshakeServer(t, time.Second)
	conn := dialWS(t, url)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, wsjson.Write(ctx, conn, registration{SessionID: "s1"}))
	require.NoError(t, <-errs)
	assert.Equal(t, "s1", (<-regs).SessionID)
}

// A silent peer is refused with the handshake's reason, a close frame and not a drop.
func TestReadFirst_NoFrameInTimeIsAPolicyViolation(t *testing.T) {
	url, errs, _ := handshakeServer(t, 50*time.Millisecond)
	conn := dialWS(t, url)
	defer conn.CloseNow()
	ce := readClose(t, conn)
	assert.Equal(t, websocket.StatusPolicyViolation, ce.Code)
	assert.Equal(t, "registration required", ce.Reason)
	err := <-errs
	assert.Contains(t, err.Error(), "no frame within", "the cause stays for the logs")
	assert.False(t, PeerGone(err), "a refused handshake is no peer gone")
}

func TestReadFirst_AnUnreadableFrameIsAPolicyViolation(t *testing.T) {
	url, errs, _ := handshakeServer(t, time.Second)
	conn := dialWS(t, url)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte("not json")))
	ce := readClose(t, conn)
	assert.Equal(t, websocket.StatusPolicyViolation, ce.Code)
	assert.Equal(t, "registration required", ce.Reason)
	require.Error(t, <-errs)
}

// A peer gone before it registered gets nothing sent: there is nobody to tell.
func TestReadFirst_APeerGoneIsReleasedQuietly(t *testing.T) {
	url, errs, _ := handshakeServer(t, time.Second)
	conn := dialWS(t, url)
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, "never mind"))
	err := <-errs
	require.Error(t, err)
	assert.True(t, PeerGone(err), "the read's own error, got %v", err)
	assert.NoError(t, Close(context.Background(), nil, err), "Close has nothing to send, nor to report")
}

// A context ending before the handshake is ctx.Err() itself, whichever wait saw it first:
// neither a peer gone nor a refusal.
func TestReadFirst_AnEndedContextIsNeitherAPeerGoneNorARefusal(t *testing.T) {
	errs, closes := make(chan error, 1), make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = ReadFirst[registration](ctx, conn, time.Second, "registration required")
		errs <- err
		closes <- Close(context.Background(), conn, err) // the server's context, still running
	}))
	t.Cleanup(server.Close)
	dialWS(t, "ws"+strings.TrimPrefix(server.URL, "http"))
	err := <-errs
	assert.Equal(t, context.Canceled, err, "ctx.Err(), not a refusal wrapping it")
	assert.False(t, PeerGone(err))
	assert.NoError(t, <-closes, "Close releases it, no failure")
}
