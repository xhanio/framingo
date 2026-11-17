package user

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/system/platform"
)

// webapp/src/main/java/com/ensilo/webapp/rest/UserRestController.java
type manager struct {
	log  log.Logger
	name string

	repository Repository
	sender     common.MessageSender
}

// New takes the platform for the messagebus the service publishes account
// changes on - deletions, password resets, identity changes - which the auth
// service ends sessions on. A platform without one drops them, and a session
// then outlives the change until its lease expires.
func New(platform platform.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

// newManager returns the concrete manager, the form package tests construct.
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
	deps := []common.Service{m.repository}
	if m.sender != nil {
		deps = append(deps, m.sender)
	}
	return deps
}
