package wsutil

import (
	"context"
	"io"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// Close ends conn once the handler that served it has returned err:
//   - nil: a normal closure;
//   - the peer's own close (normal or going away): nothing more, the
//     connection is already closing;
//   - the peer dropping the connection without a close frame (EOF): released
//     with nothing sent, there being no one left to send it to;
//   - anything else after ctx has ended: going away, the server shutting down;
//   - anything else: an internal error carrying err.
//
// Only the last is a failure, and it is the one case Close returns err, for
// the caller to log.
func Close(ctx context.Context, conn *websocket.Conn, err error) error {
	if err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
		return nil
	}
	switch status := websocket.CloseStatus(err); {
	case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
		return nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		// unlike a close frame, an EOF does not make the library close the
		// connection itself
		conn.CloseNow()
		return nil
	case ctx.Err() != nil:
		conn.Close(websocket.StatusGoingAway, "server shutting down")
		return nil
	default:
		conn.Close(websocket.StatusInternalError, err.Error())
		return err
	}
}
