package wsutil

import (
	"context"
	"io"
	"net"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"
)

// maxReason is as much of a reason as a close frame carries: coder/websocket drops a
// longer one whole, closing with an internal error and no reason.
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

// CloseWith is the error a handler returns for its connection to be closed with status
// and reason: a refusal, a displacement, a "try again later". Close sends them and
// reports no failure; the reason, sent to the peer, says no more than it should.
func CloseWith(status websocket.StatusCode, reason string) error {
	return &closeError{status: status, reason: reason}
}

// CloseWithCause is CloseWith keeping what caused it: the peer gets the reason alone,
// the error says both, for the logs, and errors.Is finds the cause. A handshake whose
// frame fails the handler's own check is refused with one, as ReadFirst refuses its own.
func CloseWithCause(status websocket.StatusCode, reason string, cause error) error {
	return &closeError{status: status, reason: reason, cause: cause}
}

// sentError is an err whose close was sent already: Close sends nothing more, and
// reports what it would have reported had it sent it.
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
// close, or EOF, nothing more; ctx done going away; a CloseWith its status and reason;
// any other error an internal error, the one case it returns, for the caller to log. A
// close the handler's Bridge sent already is not sent again.
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
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
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

// closeAs sends err's close: a CloseWith's status and reason, or else an internal error
// carrying err's message — returned then, for the caller to log.
//
//nolint:errcheck // closing is the last act on conn: a failed close has nowhere to go
func closeAs(conn *websocket.Conn, err error) error {
	var ce *closeError
	if errors.As(err, &ce) {
		conn.Close(ce.status, truncate(ce.reason))
	} else {
		conn.Close(websocket.StatusInternalError, truncate(err.Error()))
	}
	return failure(err)
}

// failure is what a close for err reports: nothing for a CloseWith, the handler asked
// for it, and err for any other.
func failure(err error) error {
	var ce *closeError
	if errors.As(err, &ce) {
		return nil
	}
	return err
}

// PeerGone reports whether err means the peer stopped talking, hanging up or sending a
// close frame on its way out: what ReadFirst returns as is, a handler telling it from a
// refused handshake to log the two apart.
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
