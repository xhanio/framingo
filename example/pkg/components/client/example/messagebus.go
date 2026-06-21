package example

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
	"github.com/xhanio/errors"

	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// StreamMessages opens a WebSocket to the /messages/stream endpoint and prints
// every PubsubMessage it receives as pretty JSON until the context is canceled
// or the connection closes.
func (c *cli) StreamMessages(ctx context.Context) error {
	wsURL, err := buildStreamURL(c.endpoint)
	if err != nil {
		return errors.Wrap(err)
	}
	header := http.Header{}
	if c.cred != nil && c.cred.SessionID != "" {
		header.Set(fapi.HeaderKeySession, c.cred.SessionID)
	}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return errors.Wrap(err)
	}
	defer conn.CloseNow()

	// On signal, send a proper close frame so the server sees a clean shutdown
	// instead of an abrupt EOF. Reads use a background context so the close
	// frame round-trip can complete; the Read loop returns when the server
	// echoes the close.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "client shutting down")
		case <-done:
		}
	}()

	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			switch websocket.CloseStatus(err) {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway:
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			return errors.Wrap(err)
		}
		var msg entity.PubsubMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Println(string(data))
			continue
		}
		out, err := json.MarshalIndent(&msg, "", "  ")
		if err != nil {
			return errors.Wrap(err)
		}
		fmt.Println(string(out))
	}
}

// buildStreamURL converts the client's HTTP endpoint (e.g. http://host/api/v1)
// into the WebSocket URL for the message stream (ws://host/api/v1/messages/stream).
func buildStreamURL(endpoint string) (string, error) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrap(err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.Newf("unsupported endpoint scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/messages/stream"
	return u.String(), nil
}
