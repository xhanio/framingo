package auth

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"

	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	log log.Logger
	um  model.UserAuthN
	lm  model.LDAPAuthN
	tm  model.APITokenAuthN

	name string

	sync.RWMutex
	users    map[string]map[string]any
	sessions map[string]*entity.Session
}

func New(platform platform.Platform, um model.UserAuthN, lm model.LDAPAuthN, tm model.APITokenAuthN, opts ...Option) Manager {
	return newManager(platform, um, lm, tm, opts...)
}

func newManager(_ platform.Platform, um model.UserAuthN, lm model.LDAPAuthN, tm model.APITokenAuthN, opts ...Option) *manager {
	m := &manager{
		um:       um,
		lm:       lm,
		tm:       tm,
		users:    make(map[string]map[string]any),
		sessions: make(map[string]*entity.Session),
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
	// m.lm, m.tm are optional
	return []common.Service{m.um}
}
