package example

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	fmodel "github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"

	"github.com/xhanio/framingo/example/pkg/services/system/platform"
)

type manager struct {
	log  log.Logger
	name string

	repository Repository
	mb         common.RawMessageSender
	meter      fmodel.Meter

	greeting string

	ctx    context.Context
	cancel context.CancelFunc
	looped chan struct{}      // closed as the run loop of the latest Start returns
	wg     syncutil.WaitGroup // work the run loop hands off, whichever Start it came from

	inst *instruments
}

func New(platform platform.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

func newManager(platform platform.Platform, repo Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
		mb:         platform.MessageBus(),
		meter:      platform.Meter(),
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
	// mb may be nil; the supervisor skips a nil dependency
	return []common.Service{m.repository, m.mb}
}
