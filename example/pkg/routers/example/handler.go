package example

import (
	"net/http"
	"reflect"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/api"
)

func (r *router) Example(c api.Context) error {
	var req api.CreateHelloWorldMessage
	if err := c.BindAny(&req); err != nil {
		return errors.BadRequest.Newf("invalid request: %v", err)
	}
	if err := c.Validate(&req); err != nil {
		return errors.Wrap(err)
	}
	body, err := r.em.HelloWorld(c, req.Message)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, body)
}

func (r *router) Handlers() map[string]any {
	handlers := make(map[string]any)
	rv := reflect.ValueOf(r)
	rt := reflect.TypeOf(r)
	// Iterate through all methods
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		// Skip the Handlers method itself
		if method.Name == "Handlers" {
			continue
		}
		// Try to convert to handler signature func(echo.Context) error
		methodValue := rv.Method(i)
		if handlerFunc, ok := methodValue.Interface().(func(echo.Context) error); ok {
			// Successfully converted - this is a handler method
			handlers[method.Name] = echo.HandlerFunc(handlerFunc)
		} else if handlerFunc, ok := methodValue.Interface().(func(api.Context) error); ok {
			handlers[method.Name] = api.WrapHandler(handlerFunc)
		}
		// If conversion fails, silently skip (not a handler method)
	}
	return handlers
}
