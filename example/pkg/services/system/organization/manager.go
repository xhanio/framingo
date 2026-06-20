package organization

import (
	"path"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
)

type manager struct {
	log  log.Logger
	name string

	repository repository.Repository
}

func New(repo repository.Repository, opts ...Option) Manager {
	m := &manager{
		repository: repo,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return []common.Service{m.repository}
}
