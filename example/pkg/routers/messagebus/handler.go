package messagebus

import (
	"fmt"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/wsutil"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

// Stream needs authnuser so the messenger has a stable identity for self-skip.
func (r *router) Stream(c api.Context, conn *websocket.Conn) error {
	session, ok := c.Session()
	if !ok || session == nil {
		return errors.Unauthorized.Newf("session required for message stream")
	}
	// unique per connection: tabs share a session, and Unsubscribe removes every subscriber by name
	messenger, err := r.mb.NewMessenger(fmt.Sprintf("ws:%s:%s", session.UID(), uuid.NewString()))
	if err != nil {
		return errors.Wrap(err)
	}
	// Bridge closes the messenger on return
	wsutil.Bridge(conn, messenger, wsutil.WithLogger(r.log))
	return nil
}
