package certificate

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

type Manager interface {
	model.Certificate
	common.Initializable
}

// Repository is the slice of the shared repository this service may touch.
type Repository interface {
	repo.Certificate
}
