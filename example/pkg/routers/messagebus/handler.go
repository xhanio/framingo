package messagebus

import (
	"fmt"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/wsutil"

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
	// The name must be unique PER CONNECTION, not per session: two tabs share
	// one session, and Unsubscribe removes every subscriber with a given name,
	// so a session-scoped name means closing one tab silently kills the other.
	messenger, err := r.mb.NewMessenger(fmt.Sprintf("ws:%s:%s", session.UID(), uuid.NewString()))
	if err != nil {
		return errors.Wrap(err)
	}
	// Bridge blocks until the connection closes and closes the
	// messenger on return — no extra cleanup needed here.
	wsutil.Bridge(conn, messenger, wsutil.WithLogger(r.log))
	return nil
}
