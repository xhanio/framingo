package user

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

type Manager interface {
	// business.go
	model.User
	model.UserAuthN
	// lifecycle.go
	common.Initializable
}

// Repository is the part of the data layer this service reads and writes
// through. The one repository instance implements every repo contract, so
// the wiring hands it over as is; this interface is what the service may touch.
type Repository interface {
	repo.User
	repo.Organization // the ID of the organization a login names
}
