package server

import (
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/api"
)

// Route is one handler a router's config declares.
type Route struct {
	Server     string
	Prefix     string // its group's
	Method     string
	Path       string // the prefix joined with the handler's own path
	Func       string
	Permission string
}

// CheckRouter parses r's config and resolves its handlers as RegisterRouters
// does, needing no server: a test calls it, so a router that would fail
// startup - a func its Handlers lack, a signature discovery skipped, a route
// declared twice - fails its tests instead. It returns what the config declares.
func CheckRouter(r api.Router) ([]Route, error) {
	groups, err := newManager().registerRouter(r)
	if err != nil {
		return nil, err
	}
	var routes []Route
	for _, g := range groups {
		if g.Server == "" {
			return nil, errors.Newf("router %s: server name not specified in handler group %q", r.Name(), g.Prefix)
		}
		for _, h := range g.Handlers {
			routes = append(routes, Route{
				Server:     g.Server,
				Prefix:     g.Prefix,
				Method:     h.Method,
				Path:       newHandlerKey(g, h).Path,
				Func:       h.Func,
				Permission: h.Permission,
			})
		}
	}
	return routes, nil
}
