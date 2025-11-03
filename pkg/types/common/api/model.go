package api

import (
	"context"
	"fmt"
	"path"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api/rbac"
)

const DefaultAPIPrefix = "/"

type Context interface {
	echo.Context
	context.Context
	Is(method, path string)
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
	Prefix      string     `json:"prefix"` // default: /
	Handlers    []*Handler `json:"handlers"`
	Middlewares []string   `json:"middlewares"`
}

type Handler struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Middlewares []string        `json:"middlewares"`
	Permission  rbac.Permission `json:"permission"`
	Poll        bool            `json:"poll"`
	Throttle    *ThrottleConfig `json:"throttle,omitempty"`
	Func        string          `json:"func"`
}

type HandlerFunc func(Context) error

func HandlerKey(g *HandlerGroup, h *Handler) string {
	prefix := DefaultAPIPrefix
	if g != nil && g.Prefix != "" {
		prefix = g.Prefix
	}
	return fmt.Sprintf("<%s>%s", h.Method, path.Join(prefix, h.Path))
}
