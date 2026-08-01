package api

import (
	"github.com/labstack/echo/v4"

	"github.com/xhanio/framingo/pkg/types/common"
)

const (
	MethodAny = "ANY"
	MethodWS  = "WS"
)

// Middleware is one of the two extension points of the API server, the mirror
// image of Router: where a Router supplies its configuration to the framework,
// a Middleware receives configuration from it.
type Middleware interface {
	common.Service
	// Func returns the middleware function for one attachment point. config is
	// the raw YAML written under the middleware's name, and nil when there is
	// none. Called once per attachment at registration time - and again on
	// restart, when routes are rebuilt - so per-route state lives in the
	// returned closure. An error fails registration; returning no function
	// and no error declines the attachment, and the server skips it.
	Func(config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error)
}

type Router interface {
	common.Service
	Config() []byte
	Handlers() map[string]any // echo.HandlerFunc or WebSocketHandlerFunc
}
