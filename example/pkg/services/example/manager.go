package example

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	fmodel "github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	log  log.Logger
	name string

	repository Repository
	mb         common.RawMessageSender
	meter      fmodel.Meter

	mu       sync.RWMutex // guards greeting: Init rewrites it, a restart again
	greeting string

	loop syncutil.Loop      // takes new work: Stop waits for it, no deadline
	wg   syncutil.WaitGroup // work the loop hands off, from any Start: Stop waits until its ctx ends

	inst *instruments
}

func New(platform model.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

func newManager(platform model.Platform, repo Repository, opts ...Option) *manager {
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
	return []common.Service{m.repository, m.mb}
}
