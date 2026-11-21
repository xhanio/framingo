package wsutil

import (
	"context"
	"io"
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

// closeAfter: the handler holds the connection until the client has read, so the read's end is
// Close's doing; leave read unset for a peer close, which leaves Close nothing to send.
func closeAfter(t *testing.T, ctx context.Context, handlerErr error, read bool) (returned, readErr error) {
	t.Helper()
	release := make(chan struct{})
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer conn.CloseNow()
		returned = Close(ctx, conn, handlerErr)
		<-release
	}))
	t.Cleanup(server.Close)

	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	if read {
		_, _, readErr = conn.Read(dialCtx)
		require.Error(t, readErr)
		require.NotErrorIs(t, readErr, context.DeadlineExceeded, "Close left the connection open")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the handler did not return")
	}
	return returned, readErr
}

func closeFrame(t *testing.T, readErr error) websocket.CloseError {
	t.Helper()
	var ce websocket.CloseError
	require.True(t, errors.As(readErr, &ce), "the client sees a close frame, got %v", readErr)
	return ce
}

func TestClose_NilIsANormalClosure(t *testing.T) {
	returned, readErr := closeAfter(t, context.Background(), nil, true)
	assert.NoError(t, returned)
	assert.Equal(t, websocket.StatusNormalClosure, closeFrame(t, readErr).Code)
}

func TestClose_HandlerErrorIsAnInternalErrorAndReturned(t *testing.T) {
	handlerErr := errors.Newf("handler broke")
	returned, readErr := closeAfter(t, context.Background(), handlerErr, true)
	assert.Equal(t, handlerErr, returned, "the failure is returned for the caller to log")
	frame := closeFrame(t, readErr)
	assert.Equal(t, websocket.StatusInternalError, frame.Code)
	assert.Equal(t, "handler broke", frame.Reason)
}

func TestClose_EndedContextIsGoingAway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	returned, readErr := closeAfter(t, ctx, errors.Newf("read: context canceled"), true)
	assert.NoError(t, returned, "a shutdown is not a failure")
	assert.Equal(t, websocket.StatusGoingAway, closeFrame(t, readErr).Code)
}

func TestClose_PeerCloseIsNotAFailure(t *testing.T) {
	for _, code := range []websocket.StatusCode{websocket.StatusNormalClosure, websocket.StatusGoingAway} {
		returned, _ := closeAfter(t, context.Background(), websocket.CloseError{Code: code}, false)
		assert.NoError(t, returned, "a close the peer began (%d) is not a failure", code)
	}
}

func TestClose_PeerDropIsReleasedNotAFailure(t *testing.T) {
	for _, dropped := range []error{io.EOF, io.ErrUnexpectedEOF, errors.Wrap(io.EOF)} {
		returned, readErr := closeAfter(t, context.Background(), dropped, true)
		assert.NoError(t, returned, "a dropped connection (%v) is not a failure", dropped)
		assert.Equal(t, websocket.StatusCode(-1), websocket.CloseStatus(readErr), "nothing is sent to a dropped peer")
	}
}

// A CloseWith says how to close, and is no failure.
func TestClose_CloseWithSendsItsStatusAndReason(t *testing.T) {
	returned, readErr := closeAfter(t, context.Background(), CloseWith(websocket.StatusPolicyViolation, "registration required"), true)
	assert.NoError(t, returned, "a close the handler asked for is not a failure")
	frame := closeFrame(t, readErr)
	assert.Equal(t, websocket.StatusPolicyViolation, frame.Code)
	assert.Equal(t, "registration required", frame.Reason)
}

// A reason longer than a close frame carries is cut, not lost: coder/websocket would
// drop it whole, and the status with it.
func TestClose_ALongReasonIsCut(t *testing.T) {
	long := strings.Repeat("é", 100) // 200 bytes, two to a rune
	returned, readErr := closeAfter(t, context.Background(), errors.Newf("%s", long), true)
	require.Error(t, returned)
	frame := closeFrame(t, readErr)
	assert.Equal(t, websocket.StatusInternalError, frame.Code)
	assert.LessOrEqual(t, len(frame.Reason), maxReason)
	assert.NotEmpty(t, frame.Reason)
	assert.True(t, strings.HasPrefix(long, frame.Reason), "cut at a rune's start")
}
