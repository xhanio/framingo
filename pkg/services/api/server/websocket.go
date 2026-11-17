package server

import (
	"context"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/wsutil"
)

// wsHandler is the signature of a `method: WS` handler.
type wsHandler = func(echo.Context, *websocket.Conn) error

// webSocket serves fn on s: it upgrades the request, runs fn on the
// connection and closes it as fn's result calls for. Once s stops, the
// connection is closed going away, which ends fn's reads, and stop waits for
// fn; one upgraded after the stop began is closed without running fn.
func (m *manager) webSocket(s *server, fn wsHandler) echo.HandlerFunc {
	return func(c echo.Context) error {
		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{InsecureSkipVerify: m.debug})
		if err != nil {
			return errors.BadRequest.Wrap(err)
		}
		ctx, ok := s.trackWebSocket()
		if !ok {
			//nolint:errcheck // nothing to do with a failure to close here
			conn.Close(websocket.StatusGoingAway, "server shutting down")
			return nil
		}
		defer s.wsWG.Done()
		defer context.AfterFunc(ctx, func() {
			//nolint:errcheck // fn's read reports how the close went
			conn.Close(websocket.StatusGoingAway, "server shutting down")
		})()
		// The response is hijacked, so fn's error cannot go back to echo.
		// ctx, not the request's, is what ends when the server stops.
		if err := wsutil.Close(ctx, conn, fn(c, conn)); err != nil {
			m.log.Error(errors.Wrap(err))
		}
		return nil
	}
}

// resetWebSocket gives a fresh echo the context its WebSocket handlers run
// under, ending the one before.
func (s *server) resetWebSocket() {
	s.Lock()
	defer s.Unlock()
	if s.wsCancel != nil {
		s.wsCancel()
	}
	s.wsCtx, s.wsCancel = context.WithCancel(context.Background())
}

// trackWebSocket counts a WebSocket handler for stop to wait on, and returns
// the context stop ends; false once it has.
func (s *server) trackWebSocket() (context.Context, bool) {
	s.Lock()
	defer s.Unlock()
	if s.wsCtx.Err() != nil {
		return nil, false
	}
	s.wsWG.Add(1)
	return s.wsCtx, true
}
