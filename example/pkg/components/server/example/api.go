package example

import (
	"github.com/xhanio/errors"
	mwexample "github.com/xhanio/framingo/example/pkg/middlewares/example"
	"github.com/xhanio/framingo/example/pkg/routers/example"
	"github.com/xhanio/framingo/pkg/types/api"
)

func (m *manager) initAPI() error {
	middlewares := []api.Middleware{
		mwexample.New(),
	}
	routers := []api.Router{
		example.New(m.example, m.log),
	}
	// register middlewares
	if err := m.api.RegisterMiddlewares(middlewares...); err != nil {
		return errors.Wrap(err)
	}
	// register routers
	if err := m.api.RegisterRouters(routers...); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
