package example

import (
	"net/http"
	"reflect"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
)

func (r *router) Example(c echo.Context) error {
	message := c.QueryParam("message")
	if message == "" {
		return errors.BadRequest.Newf("helloworld message cannot be empty!")
	}
	body, err := r.em.HelloWorld(c.Request().Context(), message)
	if err != nil {
		return errors.Wrap(err)
	}
	return c.JSON(http.StatusOK, body)
}

func (r *router) Handlers() map[string]echo.HandlerFunc {
	handlers := make(map[string]echo.HandlerFunc)
	rv := reflect.ValueOf(r)
	rt := reflect.TypeOf(r)
	// Iterate through all methods
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		// Skip the Handlers method itself
		if method.Name == "Handlers" {
			continue
		}
		// Try to convert to handler signature func(apis.Context) error
		methodValue := rv.Method(i)
		if handlerFunc, ok := methodValue.Interface().(func(echo.Context) error); ok {
			// Successfully converted - this is a handler method
			handlers[method.Name] = handlerFunc
		}
		// If conversion fails, silently skip (not a handler method)
	}
	return handlers
}
