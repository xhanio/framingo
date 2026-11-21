package user

import (
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	log  log.Logger
	name string

	repository Repository
	sender     common.MessageSender

	// bcrypt's cost; the package's tests lower it to bcrypt.MinCost, since under
	// -race one hash at the default costs about a second
	passwordCost int
	dummy        func() string // the timing decoy's hash, derived at first use at passwordCost
}

// New publishes account changes on the platform's messagebus, which auth ends
// sessions on.
func New(platform model.Platform, repo Repository, opts ...Option) Manager {
	return newManager(platform, repo, opts...)
}

func newManager(platform model.Platform, repo Repository, opts ...Option) *manager {
	m := &manager{
		repository:   repo,
		sender:       platform.MessageBus(),
		passwordCost: bcrypt.DefaultCost,
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	m.dummy = sync.OnceValue(func() string {
		h, err := m.hashPassword("dummy")
		if err != nil {
			panic(err)
		}
		return h
	})
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return []common.Service{m.repository, m.sender}
}
