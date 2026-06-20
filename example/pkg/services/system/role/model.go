package role

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type Manager interface {
	// business.go
	model.Role
	// lifecycle.go
	common.Initializable
}
