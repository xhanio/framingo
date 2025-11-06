package example

import (
	"context"
	"io"
	"path"
	"sync"

	"github.com/xhanio/framingo/example/types/entity"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

type manager struct {
	log log.Logger

	name string

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func New(opts ...Option) Manager {
	m := &manager{
		wg: &sync.WaitGroup{},
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	m.log = m.log.By(m)
	if m.ctx == nil {
		m.ctx = context.Background()
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
	return []common.Service{}
}

func (m *manager) Init() error {
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if m.cancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-ctx.Done():
				// put stop logic here
				m.log.Infof("service %s stopped", m.Name())
				return
			}
		}
	}()
	return nil
}

func (m *manager) Stop(wait bool) error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	if wait {
		m.wg.Wait()
	}
	m.cancel = nil
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	// TODO: info
}

func (m *manager) HelloWorld(ctx context.Context, message string) (*entity.Helloworld, error) {
	m.log.Info("hello world!")
	result := &entity.Helloworld{
		Message: message,
	}
	return result, nil
}
