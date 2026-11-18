package api

import (
	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/common"
)

const (
	MethodAny = "ANY"
	MethodWS  = "WS"
)

type Middleware interface {
	common.Service
	// Func is called per attachment, again on every restart, so per-route state
	// lives in the closure. enabled is the most specific boolean under the
	// middleware's name (else the entry's presence); config is the most
	// specific non-boolean YAML, resolved independently, nil when absent.
	// An error fails registration; (nil, nil) declines the attachment.
	Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error)
}

type Router interface {
	common.Service
	// Config is YAML with one handler group per `---` document, all served by Handlers.
	Config() []byte
	Handlers() map[string]any // echo.HandlerFunc or WebSocketHandlerFunc
}
