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
	return context.WithValue(ctx, common.ContextKeyConfig, v)
}
