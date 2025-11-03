package api

import (
	"context"
	"fmt"
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/framingo/pkg/types/common"
)

const DefaultAPIPrefix = "/"

type Context interface {
	echo.Context
	context.Context
}

type Middleware interface {
	common.Service
	Func(echo.HandlerFunc) echo.HandlerFunc
}

type Router interface {
	common.Service
	Handlers() map[string]echo.HandlerFunc
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

type HandlerFunc func(Context) error

func HandlerKey(prefix string, g *HandlerGroup, h *Handler) string {
	var gp string
	if g != nil && g.Prefix != "" {
		gp = g.Prefix
	}
	return fmt.Sprintf("<%s>%s", h.Method, path.Join(prefix, gp, h.Path))
}
