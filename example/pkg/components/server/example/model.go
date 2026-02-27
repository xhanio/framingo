package example

import "github.com/xhanio/framingo/pkg/types/common"

type Server interface {
	common.Named
	common.Daemon
	common.Initializable
	common.Debuggable
}
