package example

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/services/api/client"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
)

const (
	DefaultEndpoint = "http://127.0.0.1:8080/api/v1"
)

type cli struct {
	endpoint string
	opts     []client.Option
	cli      client.Client

	credFile string
	cred     *Credential
}

func New(opts ...Option) Client {
	return newClient(opts...)
}

func newClient(opts ...Option) *cli {
	c := &cli{
		cred: &Credential{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *cli) Init() error {
	if c.endpoint == "" {
		c.endpoint = DefaultEndpoint
	}
	c.cli = client.New(c.endpoint, c.opts...)
	if err := c.cli.Init(context.Background()); err != nil {
		return errors.Wrap(err)
	}
	if err := c.cred.Load(c.credFile); err != nil {
		return errors.Wrap(err)
	}
	c.cli.SetHeaders(common.NewPair(fapi.HeaderKeySession, c.cred.SessionID))
	return nil
}
