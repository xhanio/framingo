package confutil

import (
	"context"

	"github.com/spf13/viper"

	"github.com/xhanio/framingo/pkg/types/common"
)

func FromContext(ctx context.Context) *viper.Viper {
	v, ok := ctx.Value(common.ContextKeyConfig).(*viper.Viper)
	if !ok {
		return viper.New()
	}
	return v
}

func WrapContext(ctx context.Context, v *viper.Viper) context.Context {
	// Plain string key on purpose - see the note in db.WrapContext: these keys
	// are shared with echo's string-keyed request store.
	//nolint:staticcheck // SA1029: intentional, shared with echo's Get/Set
	return context.WithValue(ctx, common.ContextKeyConfig, v)
}
