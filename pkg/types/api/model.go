package api

import (
	"fmt"
	"path"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/common"
)

const (
	MethodAny = "ANY"
	MethodWS  = "WS"
)

// WebSocketHandlerFunc is a handler for WebSocket connections.
// The server upgrades the HTTP connection and passes the resulting conn.
// Returning an error closes the connection with an internal error status.
type WebSocketHandlerFunc func(c echo.Context, conn *websocket.Conn) error

type Middleware interface {
	common.Service
	Func(echo.HandlerFunc) echo.HandlerFunc
}

type Router interface {
	common.Service
	Config() []byte
	Handlers() map[string]any // echo.HandlerFunc or WebSocketHandlerFunc
}

// HandlerKey uniquely identifies a handler within a server.
type HandlerKey struct {
	Server string
	Method string
	Path   string
}

func (k HandlerKey) String() string {
	return fmt.Sprintf("[%s] %s %s", k.Server, k.Method, k.Path)
}

// NewHandlerKey creates a HandlerKey from a HandlerGroup and Handler.
func NewHandlerKey(g *HandlerGroup, h *Handler) HandlerKey {
	var server, prefix string
	if g != nil {
		server = g.Server
		prefix = g.Prefix
	}
	return HandlerKey{
		Server: server,
		Method: h.Method,
		Path:   path.Join(prefix, h.Path),
	}
}

type HandlerGroup struct {
	Server      string     `json:"server"` // default: http
	Prefix      string     `json:"prefix"` // default: /
	Handlers    []*Handler `json:"handlers"`
	Middlewares []string   `json:"middlewares"`
}

type Handler struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Middlewares []string        `json:"middlewares"`
	Permission  string          `json:"permission"`
	Poll        bool            `json:"poll"`
	Throttle    *ThrottleConfig `json:"throttle,omitempty"`
	Func        string          `json:"func"`
}
