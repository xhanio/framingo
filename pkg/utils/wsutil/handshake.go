package wsutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// ReadFirst decodes the connection's first frame, JSON, into a T — a handshake such as a
// registration — waiting no longer than timeout. Short of a frame it returns one of three
// errors, the handler's to return:
//   - the peer left first: the read's error as is, for Close to release the connection
//     quietly; PeerGone reports it.
//   - ctx ended first: ctx.Err(). The read under ctx has torn the connection down, so
//     nothing more is sent. framingo's api server never ends a WebSocket request's
//     context: on stop it closes the connection, and the read sees a peer gone.
//   - no frame in time, or one that does not decode: CloseWith(StatusPolicyViolation,
//     reason), reason naming the handshake.
//
// A handler logging refusals skips the first by PeerGone and the second by ctx.Err().
// Checking what the frame holds is the caller's, refusing one with
// CloseWithCause(StatusPolicyViolation, reason, why).
//
// The read runs on a goroutine of its own rather than under a context with the timeout,
// since coder/websocket tears the connection down when a read's context ends; after a
// timeout it stays parked until the server closes the connection. The frame is decoded
// here, not by wsjson, which closes the connection itself on a bad one.
func ReadFirst[T any](ctx context.Context, conn *websocket.Conn, timeout time.Duration, reason string) (T, error) {
	var first T
	type read struct {
		data []byte
		err  error
	}
	reads := make(chan read, 1)
	go func() {
		_, data, err := conn.Read(ctx)
		reads <- read{data, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return first, ctx.Err()
	case <-timer.C:
		return first, CloseWithCause(websocket.StatusPolicyViolation, reason, errors.Newf("no frame within %s", timeout))
	case r := <-reads:
		switch {
		case r.err != nil && ctx.Err() != nil: // the read saw ctx end before select did
			return first, ctx.Err()
		case r.err != nil && PeerGone(r.err):
			return first, r.err
		case r.err != nil:
			return first, CloseWithCause(websocket.StatusPolicyViolation, reason, r.err)
		}
		if err := json.Unmarshal(r.data, &first); err != nil {
			var zero T // what Unmarshal left half filled is not the frame
			return zero, CloseWithCause(websocket.StatusPolicyViolation, reason, errors.Wrapf(err, "unreadable first frame"))
		}
		return first, nil
	}
}
