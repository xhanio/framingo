package cmd

import (
	"context"
	"io"
	"os"
)

type Option func(*cmd)

func WithContext(ctx context.Context) Option {
	return func(c *cmd) {
		c.ctx = ctx
	}
}

func WithEnv(envs ...string) Option {
	return func(c *cmd) {
		c.envs = envs
	}
}

func Print() Option {
	return func(c *cmd) {
		c.print = true
	}
}

func Async() Option {
	return func(c *cmd) {
		c.async = true
	}
}

func WithInput(inputs ...io.Reader) Option {
	return func(c *cmd) {
		if len(inputs) == 0 {
			c.in = os.Stdin
		} else {
			c.in = io.MultiReader(inputs...)
		}
	}
}
