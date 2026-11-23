package server

import (
	"context"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/wsutil"
)

type wsHandler = func(echo.Context, *websocket.Conn) error

// once s stops the connection is closed going away and stop waits for fn; one upgraded after stop began never runs fn
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
		// hijacked: fn's error cannot go back to echo; ctx, not the request's, ends on stop
		// only an error the handler did not mean comes back: this is its one report
		if err := wsutil.Close(ctx, conn, fn(c, conn)); err != nil {
			m.log.Errorf("websocket %s failed: %+v", c.Path(), err)
		}
		return nil
	}
}

func (s *server) resetWebSocket() {
	s.Lock()
	defer s.Unlock()
	if s.wsCancel != nil {
		s.wsCancel()
	}
	s.wsCtx, s.wsCancel = context.WithCancel(context.Background())
}

// false once stop has begun
func (s *server) trackWebSocket() (context.Context, bool) {
	s.Lock()
	defer s.Unlock()
	if s.wsCtx.Err() != nil {
		return nil, false
	}
	s.wsWG.Add(1)
	return s.wsCtx, true
}
