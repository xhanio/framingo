package vdb

import (
	"context"
	"path"

	"github.com/milvus-io/milvus-sdk-go/v2/client"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	name string
	log  log.Logger

	source Source

	c client.Client
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Init() error {
	return nil
}

func (m *manager) Start() error {
	c, err := client.NewClient(context.Background(), client.Config{
		Address:  m.source.Host,
		Username: m.source.User,
		Password: m.source.Password,
		DBName:   m.source.DBName,
	})
	if err != nil {
		return errors.Wrap(err)
	}
	m.c = c
	return nil
}

func (m *manager) Stop(wait bool) error {
	return m.c.Close()
}

func (m *manager) Client() client.Client {
	return m.c
}
