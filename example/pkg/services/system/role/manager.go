package role

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/repository"
)

type manager struct {
	name string
	log  log.Logger

	repository repository.Repository
}

func New(repo repository.Repository, opts ...Option) Manager {
	return newManager(repo, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(repo repository.Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)

	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return []common.Service{m.repository}
}
