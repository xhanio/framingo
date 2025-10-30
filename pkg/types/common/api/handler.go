package api

import (
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/xhanio/framingo/pkg/types/common/api/rbac"
)

type Router struct {
	Name        string                `json:"name"`
	Prefix      string                `json:"prefix"`
	Secure      bool                  `json:"secure"`
	Handlers    []*Handler            `json:"handlers"`
	Middlewares []echo.MiddlewareFunc `json:"middlewares"`
}

type Handler struct {
	Method            string                `json:"method"`
	Path              string                `json:"path"`
	Middlewares       []echo.MiddlewareFunc `json:"middlewares"`
	Permission        rbac.Permission       `json:"permission"`
	Poll              bool                  `json:"poll"`
	Handler           HandlerFunc           `json:"handler"`
	ThrottleRPS       rate.Limit            `json:"throttle_rps"`
	ThrottleBurstSize int                   `json:"throttle_burst_size"`
}

type HandlerFunc func(Context) error

type HandlerWrapper func(HandlerFunc) echo.HandlerFunc
