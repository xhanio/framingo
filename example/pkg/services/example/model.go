package example

import (
	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/model"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

type Manager interface {
	// business.go
	model.Example
	// lifecycle.go
	common.Initializable
	common.Debuggable
	common.Daemon
}

// Repository is the part of the data layer this service reads and writes
// through. The one repository instance implements every repo contract, so
// the wiring hands it over as is; this interface is what the service may touch.
type Repository interface {
	repo.HelloWorld
}
