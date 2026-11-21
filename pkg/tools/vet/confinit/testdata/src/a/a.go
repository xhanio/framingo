package a

import (
	"context"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

type manager struct{ greeting string }

func (m *manager) Init(ctx context.Context) error {
	m.greeting = confutil.FromContext(ctx).GetString("greeting")
	return nil
}

func (m *manager) Hello(ctx context.Context) string {
	return confutil.FromContext(ctx).GetString("greeting") // want `confutil.FromContext outside Init`
}

func helper(ctx context.Context) string {
	return confutil.FromContext(ctx).GetString("x") // want `confutil.FromContext outside Init`
}

// a function named Init that is no method is not the lifecycle's
func Init(ctx context.Context) string {
	return confutil.FromContext(ctx).GetString("x") // want `confutil.FromContext outside Init`
}

func (m *manager) Migrate(ctx context.Context) string {
	//framingo:ignore confinit a one-off command, with no Init to read in
	return confutil.FromContext(ctx).GetString("dir")
}
