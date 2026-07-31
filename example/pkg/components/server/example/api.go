package example

import (
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/api"

	"github.com/xhanio/framingo/example/pkg/middlewares/authnuser"
	"github.com/xhanio/framingo/example/pkg/middlewares/authz"
	"github.com/xhanio/framingo/example/pkg/middlewares/deflate"
	"github.com/xhanio/framingo/example/pkg/middlewares/throttle"
	authRouter "github.com/xhanio/framingo/example/pkg/routers/auth"
	certRouter "github.com/xhanio/framingo/example/pkg/routers/certificate"
	exampleRouter "github.com/xhanio/framingo/example/pkg/routers/example"
	messagebusRouter "github.com/xhanio/framingo/example/pkg/routers/messagebus"
	roleRouter "github.com/xhanio/framingo/example/pkg/routers/role"
	userRouter "github.com/xhanio/framingo/example/pkg/routers/user"
)

func (m *manager) initAPI() error {
	middlewares := []api.Middleware{
		deflate.New(),
		authnuser.New(m.auth),
		authz.New(m.role),
		// Routers opt in through router.yaml, where a handler may also carry
		// its own limit under the middleware's name; this instance limit
		// covers the rest. Built without one it passes everything, so
		// attaching it is safe with no config at all.
		throttle.New(throttle.WithLimit(
			m.config.GetFloat64("api.http.throttle.rps"),
			m.config.GetInt("api.http.throttle.burst_size"),
		)),
	}
	routers := []api.Router{
		exampleRouter.New(m.example, m.log),
		authRouter.New(m.auth, m.role, m.log),
		userRouter.New(m.user, m.role, m.auth, m.log),
		roleRouter.New(m.role, m.log),
		certRouter.New(m.certificate, m.log),
		messagebusRouter.New(m.messagebus, m.log),
	}
	if err := m.api.RegisterMiddlewares(middlewares...); err != nil {
		return errors.Wrap(err)
	}
	if err := m.api.RegisterRouters(routers...); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
