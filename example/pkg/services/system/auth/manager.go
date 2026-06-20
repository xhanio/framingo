package auth

import (
	"path"
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

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

func New(um model.UserAuthN, lm model.LDAPAuthN, tm model.APITokenAuthN, opts ...Option) Manager {
	return newManager(um, lm, tm, opts...)
}

func newManager(um model.UserAuthN, lm model.LDAPAuthN, tm model.APITokenAuthN, opts ...Option) *manager {
	m := &manager{
		um:       um,
		lm:       lm,
		tm:       tm,
		users:    make(map[string]map[string]any),
		sessions: make(map[string]*entity.Session),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	// m.lm, m.tm are optional
	return []common.Service{m.um}
}
