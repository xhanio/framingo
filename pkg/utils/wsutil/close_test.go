package wsutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
)

// closeAfter serves one connection whose handler returned handlerErr under
// ctx, and reports what Close returned and the close status the client saw;
// read is false when the client is not to read (the handler's error was the
// peer's own close, so Close sends nothing to wait for).
func closeAfter(t *testing.T, ctx context.Context, handlerErr error, read bool) (returned error, status websocket.StatusCode, reason string) {
	t.Helper()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		//nolint:errcheck // a no-op once Close has closed it
		defer conn.CloseNow()
		returned = Close(ctx, conn, handlerErr)
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	//nolint:errcheck // nothing to do with a failure to close here
	defer conn.CloseNow()

	if read {
		_, _, err = conn.Read(dialCtx)
		var ce websocket.CloseError
		require.True(t, errors.As(err, &ce), "the client sees a close frame, got %v", err)
		status, reason = ce.Code, ce.Reason
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the handler did not return")
	}
	return returned, status, reason
}

func TestClose_NilIsANormalClosure(t *testing.T) {
	returned, status, _ := closeAfter(t, context.Background(), nil, true)
	assert.NoError(t, returned)
	assert.Equal(t, websocket.StatusNormalClosure, status)
}

func TestClose_HandlerErrorIsAnInternalErrorAndReturned(t *testing.T) {
	handlerErr := errors.Newf("handler broke")
	returned, status, reason := closeAfter(t, context.Background(), handlerErr, true)
	assert.Equal(t, handlerErr, returned, "the failure is returned for the caller to log")
	assert.Equal(t, websocket.StatusInternalError, status)
	assert.Equal(t, "handler broke", reason)
}

func TestClose_EndedContextIsGoingAway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	returned, status, _ := closeAfter(t, ctx, errors.Newf("read: context canceled"), true)
	assert.NoError(t, returned, "a shutdown is not a failure")
	assert.Equal(t, websocket.StatusGoingAway, status)
}

func TestClose_PeerCloseIsNotAFailure(t *testing.T) {
	for _, code := range []websocket.StatusCode{websocket.StatusNormalClosure, websocket.StatusGoingAway} {
		returned, _, _ := closeAfter(t, context.Background(), websocket.CloseError{Code: code}, false)
		assert.NoError(t, returned, "a close the peer began (%d) is not a failure", code)
	}
}
