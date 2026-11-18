package confutil

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

// FromContext never returns nil: without a config every getter yields zero, so validate mandatory keys in Init.
func FromContext(ctx context.Context) Config {
	c, ok := ctx.Value(common.ContextKeyConfig).(Config)
	if !ok {
		return New(nil)
	}
	return c
}

func WrapContext(ctx context.Context, c Config) context.Context {
	//nolint:staticcheck // SA1029: string key on purpose, shared with echo's Get/Set
	return context.WithValue(ctx, common.ContextKeyConfig, c)
}
