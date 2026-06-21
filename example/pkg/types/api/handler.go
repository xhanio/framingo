package api

import (
	"reflect"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
)

type WebSocketFunc func(Context, *websocket.Conn) error

func WrapWebsocket(wf WebSocketFunc) func(echo.Context, *websocket.Conn) error {
	return func(c echo.Context, conn *websocket.Conn) error {
		return wf(&ctx{c}, conn)
	}
}

type HandlerFunc func(Context) error

func WrapHandler(hf HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return hf(&ctx{c})
	}
}

// DiscoverHandlers reflects over r's methods and returns those matching a
// known handler signature, keyed by method name. Methods named "Handlers"
// are skipped so a router's own Handlers() method doesn't recurse into the map.
func DiscoverHandlers(r any) map[string]any {
	handlers := make(map[string]any)
	rv := reflect.ValueOf(r)
	rt := reflect.TypeOf(r)
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		if method.Name == "Handlers" {
			continue
		}
		switch fn := rv.Method(i).Interface().(type) {
		case func(echo.Context) error:
			handlers[method.Name] = echo.HandlerFunc(fn)
		case func(Context) error:
			handlers[method.Name] = WrapHandler(fn)
		case func(Context, *websocket.Conn) error:
			handlers[method.Name] = WrapWebsocket(fn)
		}
	}
	return handlers
}
