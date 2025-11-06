package vdb

import (
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/xhanio/framingo/pkg/types/common"
)

type Source struct {
	Host     string
	Port     uint
	User     string
	Password string `print:"-"`
	DBName   string
}

type Manager interface {
	common.Service
	common.Initializable
	Client() client.Client
}
