package example

import (
	"context"

	"github.com/xhanio/framingo/example/types/entity"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Manager interface {
	common.Service
	common.Initializable
	common.Debuggable
	common.Daemon
	HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error)
}
