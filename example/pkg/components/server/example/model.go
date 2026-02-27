package example

import "github.com/xhanio/framingo/pkg/types/common"

type Server interface {
	common.Daemon
	common.Initializable
	common.Debuggable
}
