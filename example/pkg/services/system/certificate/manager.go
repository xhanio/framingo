package certificate

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	name string
	log  log.Logger

	repository Repository

	mu       sync.Mutex // guards the two CAs: Init writes them, a restart again
	serverCA certutil.CABundle
	agentCA  certutil.CABundle
}

func New(platform model.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

func newManager(_ model.Platform, repo Repository, opts ...Option) *manager {
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
