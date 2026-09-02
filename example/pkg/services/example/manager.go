package example

import (
	"context"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/repository"
)

type manager struct {
	log  log.Logger
	name string

	repository repository.Repository
	mb         common.RawMessageSender

	greeting string

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	metrics *metric
}

func New(infra Infra, repo repository.Repository, opts ...Option) Manager {
	return newManager(infra, repo, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(infra Infra, repo repository.Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
		mb:         infra.MessageBus(),
		wg:         &sync.WaitGroup{},
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	m.metrics = m.newMetric(infra.Meter())
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	deps := []common.Service{m.repository}
	if s, ok := m.mb.(common.Service); ok {
		deps = append(deps, s)
	}
	return deps
}
