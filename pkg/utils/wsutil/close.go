package wsutil

import (
	"context"
	"io"
	"net"
	"net/http"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"

	fapi "github.com/xhanio/framingo/pkg/types/api"
)

// maxReason is as long as a close frame's reason may be: coder/websocket drops a longer
// one whole, closing with an internal error and no reason.
const maxReason = 123

// closeError is a close a handler asks for: its status and reason, and what caused it.
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

// CloseWith is the error a handler returns to have its connection closed with status
// and reason — a refusal, a displacement, a "try again later". Close sends them and
// reports no failure. The peer reads the reason: keep out of it what it should not.
func CloseWith(status websocket.StatusCode, reason string) error {
	return &closeError{status: status, reason: reason}
}

// CloseWithCause is CloseWith keeping its cause: the peer gets the reason alone, the
// error's message says both for the logs, and errors.Is finds the cause.
func CloseWithCause(status websocket.StatusCode, reason string, cause error) error {
	return &closeError{status: status, reason: reason, cause: cause}
}

// sentError marks a close already sent: Close sends nothing more, and reports what it
// would have reported.
type sentError struct {
	err error
}

func (e *sentError) Error() string { return e.err.Error() }

func (e *sentError) Unwrap() error { return e.err }

// sent marks err's close as sent.
func sent(err error) error {
	return &sentError{err: err}
}

// Close closes conn as the handler's err says: nil a normal closure; the peer's own
// close, an EOF, or a context's end — taken for a read's, which tore the connection
// down — nothing more; ctx done going away; a CloseWith its status and reason, whatever
// its cause; any other error an internal error, the one case it returns, for the caller
// to log. A close the handler's Bridge sent already is not sent again.
//
//nolint:errcheck // closing is the last act on conn: a failed close has nowhere to go
func Close(ctx context.Context, conn *websocket.Conn, err error) error {
	if err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
		return nil
	}
	var done *sentError
	if errors.As(err, &done) {
		return failure(done.err)
	}
	switch status := websocket.CloseStatus(err); {
	case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
		return nil
	case released(err):
		// the library does not close the connection itself on EOF
		conn.CloseNow()
		return nil
	case ctx.Err() != nil:
		conn.Close(websocket.StatusGoingAway, "server shutting down")
		return nil
	default:
		return closeAs(conn, err)
	}
}

// released reports whether err leaves nothing to send: the peer dropped the connection,
// or a read's context ended. A CloseWith is never one, whatever its cause.
func released(err error) bool {
	var ce *closeError
	if errors.As(err, &ce) {
		return false
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// closeAs sends err's close: a CloseWith's status and reason, or else an internal error
// carrying what a 500 would tell the client (fapi.PublicMessage) — err returned then,
// whole, for the caller to log.
//
//nolint:errcheck // closing is the last act on conn: a failed close has nowhere to go
func closeAs(conn *websocket.Conn, err error) error {
	var ce *closeError
	if errors.As(err, &ce) {
		conn.Close(ce.status, truncate(ce.reason))
	} else {
		conn.Close(websocket.StatusInternalError, truncate(fapi.PublicMessage(err, http.StatusInternalServerError)))
	}
	return failure(err)
}

// failure is what Close reports for err: nil for a CloseWith, which the handler asked
// for, and err otherwise.
func failure(err error) error {
	var ce *closeError
	if errors.As(err, &ce) {
		return nil
	}
	return err
}

// PeerGone reports whether err means the connection went under a read: the peer hung up
// or sent a close frame, or the server closed the connection. A handler tells it from a
// refused handshake by PeerGone, to log the two apart; a context's end is neither, and
// ReadFirst returns it as ctx.Err().
func PeerGone(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		websocket.CloseStatus(err) != -1
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
