package confutil

import "context"

type Config interface{ GetString(key string) string }

func FromContext(ctx context.Context) Config { return nil }
