package example

import (
	"github.com/xhanio/framingo/pkg/types/common"
	fmodel "github.com/xhanio/framingo/pkg/types/model"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

// Platform is this service's narrow view of the app's platform bundle.
type Platform interface {
	Meter() fmodel.Meter
	MessageBus() fmodel.MessageBus
}

type Manager interface {
	// business.go
	model.Example
	// lifecycle.go
	common.Initializable
	common.Debuggable
	common.Daemon
}
