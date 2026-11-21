package a

import (
	"context"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// a test reads the config it wrapped
func read(ctx context.Context) string {
	return confutil.FromContext(ctx).GetString("greeting")
}
