package example

import "github.com/xhanio/framingo/pkg/types/common"

type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	common.Debuggable
}
