package example

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type Manager interface {
	// business.go
	model.Example
	// lifecycle.go
	common.Initializable
	common.Debuggable
	common.Daemon
}
