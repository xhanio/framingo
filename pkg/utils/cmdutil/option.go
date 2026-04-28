package cmdutil

import (
	"context"
	"io"
	"os"
	"time"
)

type Option func(*cmd)

func (c *cmd) apply(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

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

func WithDir(dir string) Option {
	return func(c *cmd) {
		c.dir = dir
	}
}

func WithCancel(fn func(*os.Process) error) Option {
	return func(c *cmd) {
		c.cancel = fn
	}
}

func WithWaitDelay(d time.Duration) Option {
	return func(c *cmd) {
		c.waitDelay = d
	}
}

func WithMaxBuffer(n int) Option {
	return func(c *cmd) {
		c.maxBuffer = n
	}
}

func WithStdout(w io.Writer) Option {
	return func(c *cmd) {
		c.stdout = w
	}
}

func WithStderr(w io.Writer) Option {
	return func(c *cmd) {
		c.stderr = w
	}
}
