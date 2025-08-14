package vdb

import (
	"github.com/xhanio/framingo/pkg/types/common"
)

type Manager interface {
	common.Service
	common.Initializable
}
