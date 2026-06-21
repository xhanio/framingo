package example

import (
	"net/http"

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
	return api.DiscoverHandlers(r)
}
