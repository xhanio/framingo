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
	// Func returns the middleware function for one attachment point.
	//
	// enabled is the resolved switch for the attachment: booleans written
	// under the middleware's name are switches, not config, and the most
	// specific one wins - a handler entry over its group's over the server's
	// mapping. With no boolean anywhere, a route attachment is enabled by
	// the router.yaml entry's presence, and a server-level attachment
	// (WithMiddlewares) is enabled by its entry in the server's middleware
	// configs - no entry leaves it dormant. The standard first line is
	// `if !enabled { return nil, nil }`.
	//
	// config is the most specific non-boolean YAML written under the
	// middleware's name, nil when there is none - config and switch resolve
	// independently, so a handler's `true` can re-enable an attachment while
	// still inheriting the group's block. Called once per attachment at
	// registration time - and again on restart, when routes are rebuilt - so
	// per-route state lives in the returned closure. An error fails
	// registration; returning no function and no error declines the
	// attachment, and the server skips it.
	Func(enabled bool, config []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error)
}

type Router interface {
	common.Service
	Config() []byte
	Handlers() map[string]any // echo.HandlerFunc or WebSocketHandlerFunc
}
