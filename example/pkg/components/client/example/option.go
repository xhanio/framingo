package example

import (
	"github.com/xhanio/framingo/pkg/services/api/client"
)

type Option func(*cli)

func WithEndpoint(endpoint string) Option {
	return func(c *cli) {
		c.endpoint = endpoint
	}
}

func WithDebug() Option {
	return func(c *cli) {
		c.opts = append(c.opts, client.WithDebug())
	}
}

func WithCredential(credFile string) Option {
	return func(c *cli) {
		c.credFile = credFile
	}
}
