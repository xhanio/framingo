package task

import (
	"context"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

var _ Manager = (*manager)(nil)

type manager struct {
	log log.Logger

	name string

	cm   *cron.Cron
	pq   staque.Priority[*registration]
	wake *registration // what Stop pushes for the loop waiting on an empty queue

	concurrent int
	el         *sync.RWMutex // lock for registered and running
	registered map[string]*registration
	running    map[*entity.Task]*execution
	arrivals   uint64             // numbers each entry into the queue, for first come first served
	executions syncutil.WaitGroup // whichever Start they came from

	cancel context.CancelFunc
	looped chan struct{} // closed as the loop of the latest Start returns
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		wake:       &registration{},
		el:         &sync.RWMutex{},
		registered: make(map[string]*registration),
		running:    make(map[*entity.Task]*execution),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.pq = staque.NewPriority(
		staque.WithLessFunc(queueOrder),
		staque.WithLogger[*registration](m.log),
		staque.BlockIfEmpty[*registration](),
	)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
