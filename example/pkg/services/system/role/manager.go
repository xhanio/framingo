package role

import (
	"path"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
)

type manager struct {
	name string
	log  log.Logger

	repository repository.Repository

	handlerInfo []handlerPermission
}

type handlerPermission struct {
	Action     string
	Resource   string
	Permission string
}

func New(repo repository.Repository, opts ...Option) Manager {
	return newRole(repo, opts...)
}

// For test use
func newRole(repo repository.Repository, opts ...Option) *manager {
	m := &manager{
		repository:  repo,
		handlerInfo: make([]handlerPermission, 10),
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
	return []common.Service{m.repository}
}
