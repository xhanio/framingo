package wsutil

import (
	"context"
	"io"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// Close closes conn per the handler's err (nil, peer close, EOF, ctx done: no failure); it returns
// err only for an otherwise unexplained error, sent as an internal error.
//
//nolint:errcheck // closing is the last act on conn: a failed close has nowhere to go
func Close(ctx context.Context, conn *websocket.Conn, err error) error {
	if err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
		return nil
	}
	switch status := websocket.CloseStatus(err); {
	case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
		return nil
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		// the library does not close the connection itself on EOF
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
