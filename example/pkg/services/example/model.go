package example

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

type Manager interface {
	model.Example
	common.Initializable
	common.Debuggable
	common.Daemon
}

// Repository is the slice of the shared repository this service may touch.
type Repository interface {
	repo.HelloWorld
}
