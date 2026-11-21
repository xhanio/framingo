package wsutil

import (
	"context"
	"io"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// maxReason is as much of a reason as a close frame carries: coder/websocket drops a
// longer one whole, closing with an internal error and no reason.
const maxReason = 123

// closeError says how a handler wants its connection closed.
type closeError struct {
	status websocket.StatusCode
	reason string
	cause  error
}

func (e *closeError) Error() string {
	if e.cause != nil {
		return e.reason + ": " + e.cause.Error()
	}
	return e.reason
}

func (e *closeError) Unwrap() error { return e.cause }

// CloseWith is the error a handler returns for its connection to be closed with status
// and reason: a refusal, a displacement, a "try again later". Close sends them and
// reports no failure; the reason, sent to the peer, says no more than it should.
func CloseWith(status websocket.StatusCode, reason string) error {
	return &closeError{status: status, reason: reason}
}

// Close closes conn per the handler's err: nil a normal closure; the peer's own close,
// or EOF, nothing more; ctx done going away; a CloseWith its status and reason; any other
// error an internal error, the one case it returns, for the caller to log.
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
		return closeBy(conn, err)
	}
}

// closeBy sends err's close: a CloseWith's status and reason, an internal error carrying
// err's message otherwise — returned then, for the caller to log.
//
//nolint:errcheck // closing is the last act on conn: a failed close has nowhere to go
func closeBy(conn *websocket.Conn, err error) error {
	var ce *closeError
	if errors.As(err, &ce) {
		conn.Close(ce.status, truncate(ce.reason))
		return nil
	}
	conn.Close(websocket.StatusInternalError, truncate(err.Error()))
	return err
}

// truncate cuts reason to maxReason bytes, at a rune's start.
func truncate(reason string) string {
	if len(reason) <= maxReason {
		return reason
	}
	cut := maxReason
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return reason[:cut]
}
