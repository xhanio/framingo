package user

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/system/platform"
)

type manager struct {
	log  log.Logger
	name string

	repository Repository
	sender     common.MessageSender
}

// New publishes account changes on the platform's messagebus for auth to end
// sessions; without one, sessions outlive the change until their lease expires.
func New(platform platform.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

func newManager(platform platform.Platform, repo Repository, opts ...Option) *manager {
	m := &manager{
		repository: repo,
		sender:     platform.MessageBus(),
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
	// sender may be nil; the supervisor skips a nil dependency
	return []common.Service{m.repository, m.sender}
}
