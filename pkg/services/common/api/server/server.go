package server

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type server struct {
	name string
	log  log.Logger

	httpEndpoint   *api.Endpoint
	httpsEndpoint  *api.Endpoint
	tlsConfig      *api.ServerTLS
	throttleConfig *api.ThrottleConfig

	http  *echo.Echo
	https *echo.Echo

	handlers map[string]*api.Handler
	routers  map[string]*api.Router

	sync.Mutex // lock for rate limiters
	limits     map[string]*rate.Limiter

	handlerWrapper api.HandlerWrapper
}

func New(opts ...Option) Server {
	return newServer(opts...)
}

func newServer(opts ...Option) *server {
	s := &server{
		handlers:       make(map[string]*api.Handler),
		routers:        make(map[string]*api.Router),
		limits:         make(map[string]*rate.Limiter),
		handlerWrapper: api.DefaultHandlerWrapper,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = log.Default
	}
	return s
}

func (s *server) newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = s.ErrorHandler
	return e
}

func (s *server) Name() string {
	if s.name == "" {
		s.name = path.Join(reflectutil.Locate(s))
	}
	return s.name
}

func (s *server) Dependencies() []common.Service {
	return nil
}

func (s *server) Init() error {
	s.http = s.newEcho()
	s.https = s.newEcho()
	s.RegisterMiddlewares(
		s.Recover,
		s.Logger,
		s.Info,
		s.Error,
		s.Throttle,
	)
	return nil
}

func (s *server) RegisterRouters(routers []*api.Router, middlewares ...echo.MiddlewareFunc) error {
	for _, r := range routers {
		s.log.Debugf("registering router %s", r.Name)
		prefix, group := s.routerGroup(r)
		for _, h := range r.Handlers {
			var mw []echo.MiddlewareFunc
			mw = append(mw, h.Middlewares...)
			mw = append(mw, r.Middlewares...)
			mw = append(mw, middlewares...)
			group.Add(h.Method, h.Path, s.handlerWrapper(h.Handler), mw...)
			// register API key to retrieve its handler and router
			key := fmt.Sprintf("%s:%s", h.Method, path.Join(prefix, h.Path))
			s.handlers[key] = h
			s.routers[key] = r
		}
	}
	return nil
}

func (s *server) RegisterMiddlewares(middlewares ...echo.MiddlewareFunc) {
	s.http.Use(middlewares...)
	s.https.Use(middlewares...)
}

func (s *server) ServerPrefix(router *api.Router) string {
	var prefix string
	if router.Secure {
		if s.httpsEndpoint != nil {
			prefix = path.Join(s.httpsEndpoint.Path, router.Prefix)
		}
	} else {
		if s.httpEndpoint != nil {
			prefix = path.Join(s.httpEndpoint.Path, router.Prefix)
		}
	}
	if prefix == "" {
		return "/"
	}
	return prefix
}

func (s *server) routerGroup(router *api.Router) (string, *echo.Group) {
	prefix := s.ServerPrefix(router)
	var group *echo.Group
	if router.Secure {
		group = s.https.Group(prefix)
	} else {
		group = s.http.Group(prefix)
	}
	return prefix, group
}

func (s *server) startHTTP() error {
	if s.httpEndpoint == nil {
		return nil
	}
	s.log.Infof("serves http on %s", s.httpEndpoint.String())
	return s.http.Start(s.httpEndpoint.Address())
}

func (s *server) startHTTPS() error {
	if s.httpsEndpoint == nil {
		return nil
	}
	s.https.TLSServer = &http.Server{
		Addr:      s.httpsEndpoint.Address(),
		TLSConfig: s.tlsConfig.AsConfig(),
	}
	s.log.Infof("serves https on %s", s.httpsEndpoint.String())
	return s.https.StartServer(s.https.TLSServer)
}

func (s *server) Start(ctx context.Context) error {
	// TODO: find a better way
	go func() {
		if err := s.startHTTP(); err != nil {
			s.log.Debugf("startHTTP returns err: %v", err)
		}
	}()
	go func() {
		if err := s.startHTTPS(); err != nil {
			s.log.Debugf("startHTTPS returns err: %v", err)
		}
	}()
	return nil
}

func (s *server) Stop(wait bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		return errors.Wrap(err)
	}
	if err := s.https.Shutdown(ctx); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
