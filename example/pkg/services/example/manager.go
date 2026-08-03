package example

import (
	"context"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"

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
}

func New(repo repository.Repository, mb common.RawMessageSender, opts ...Option) Manager {
	return newManager(repo, mb, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(repo repository.Repository, mb common.RawMessageSender, opts ...Option) *manager {
	m := &manager{
		log:        log.Default,
		repository: repo,
		mb:         mb,
		wg:         &sync.WaitGroup{},
	}
	m.apply(opts...)
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
	m.log = m.log.By(m)
	if m.ctx == nil {
		m.ctx = context.Background()
	}
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
