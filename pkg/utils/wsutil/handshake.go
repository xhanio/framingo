package wsutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// ReadFirst reads the connection's first frame, JSON, into a T: the handshake a client
// begins with, a registration say, waited for no longer than timeout.
//
// A peer gone before it sent one returns what the read said, for Close to release the
// connection quietly; none in time, or one that does not decode, returns a
// CloseWith(StatusPolicyViolation, reason), reason naming the handshake. PeerGone tells
// the two apart. What the frame must hold is the caller's to check, returning a
// CloseWithCause(StatusPolicyViolation, reason, why) as well.
//
// The read waits on a goroutine of its own, not on a context with the timeout:
// coder/websocket tears the connection down when a read's context ends, and the peer
// would see a drop instead of the reason. On a timeout that read stays parked until the
// server closes the connection, once the handler returns. The frame is decoded here, not
// by wsjson, which closes the connection itself on a bad one.
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
