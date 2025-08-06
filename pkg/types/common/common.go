package common

import (
	"context"
	"io"
)

type Named interface {
	Name() string
}

type Service interface {
	Named
	Dependencies() []Service
}

type Daemon interface {
	Start(ctx context.Context) error
	Stop(wait bool) error
}

type Initializable interface {
	Init() error
}

type Debuggable interface {
	Info(w io.Writer, debug bool)
}

type Unique interface {
	Key() string
}

type Weighted interface {
	GetPriority() int
	SetPriority(priority int)
}
