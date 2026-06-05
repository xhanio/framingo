package driver

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

// Driver defines the interface for subscription storage and event delivery.
type Driver interface {
	common.Daemon
	model.PubsubDriver
}
