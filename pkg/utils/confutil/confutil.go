package confutil

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

// FromContext returns the Config the supervisor wrapped into the Init
// context. It never returns nil: with no config in the context it returns
// an empty Config, so every getter yields the zero value - a missing config
// looks like "all defaults", so a mandatory setting is validated in Init.
func FromContext(ctx context.Context) Config {
	c, ok := ctx.Value(common.ContextKeyConfig).(Config)
	if !ok {
		return New(nil)
	}
	return c
}

func WrapContext(ctx context.Context, c Config) context.Context {
	// Plain string key on purpose - see the note in db.WrapContext: these keys
	// are shared with echo's string-keyed request store.
	//nolint:staticcheck // SA1029: intentional, shared with echo's Get/Set
	return context.WithValue(ctx, common.ContextKeyConfig, c)
}
