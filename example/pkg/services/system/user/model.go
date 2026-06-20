package user

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type Manager interface {
	// business.go
	model.User
	model.UserAuthN
	// lifecycle.go
	common.Initializable
}
