package example

import (
	"context"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	fmodel "github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
)

type manager struct {
	log  log.Logger
	name string

	repository repository.Repository
	mb         common.RawMessageSender
	meter      fmodel.Meter

	greeting string

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	metrics *metrics
}

func New(platform platform.Platform, repo repository.Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
func newManager(platform platform.Platform, repo repository.Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
		mb:         platform.MessageBus(),
		meter:      platform.Meter(),
		wg:         &sync.WaitGroup{},
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
	deps := []common.Service{m.repository}
	if s, ok := m.mb.(common.Service); ok {
		deps = append(deps, s)
	}
	return deps
}
