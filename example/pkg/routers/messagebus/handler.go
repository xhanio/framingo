package messagebus

import (
	"fmt"

	"github.com/coder/websocket"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// Stream upgrades the request to a WebSocket and bridges it to the message
// bus: outbound messages flow bus → ws, inbound JSON frames flow ws → bus.
// Requires an authenticated session (authnuser middleware) so the messenger
// has a stable identity for self-skip semantics.
func (r *router) Stream(c api.Context, conn *websocket.Conn) error {
	session, ok := c.Session()
	if !ok || session == nil {
		return errors.Unauthorized.Newf("session required for message stream")
	}
	messenger, err := r.mb.NewMessenger(fmt.Sprintf("ws:%s", session.UID()))
	if err != nil {
		return errors.Wrap(err)
	}
	// AttachWebSocket blocks until the connection closes and closes the
	// messenger on return — no extra cleanup needed here.
	r.mb.AttachWebSocket(messenger, conn)
	return nil
}

func (r *router) Handlers() map[string]any {
	return api.DiscoverHandlers(r)
}
