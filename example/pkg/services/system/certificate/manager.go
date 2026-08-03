package certificate

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
)

type manager struct {
	name string
	log  log.Logger

	repository repository.Repository
	ca         certutil.CABundle
}

func New(repo repository.Repository, opts ...Option) Manager {
	return newManager(repo, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(repo repository.Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.name = nameutil.Name(m)
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return []common.Service{m.repository}
}
