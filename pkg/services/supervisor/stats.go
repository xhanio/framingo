package supervisor

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// statsStore: the monitor writes while health endpoints read; access only via
// track, snapshot and update.
type statsStore struct {
	sync.RWMutex
	records map[string]*entity.SupervisorStats
}

func newStatsStore() *statsStore {
	return &statsStore{
		records: make(map[string]*entity.SupervisorStats),
	}
}

// track returns the service already holding the name, nil when newly added.
func (s *statsStore) track(service common.Service) common.Service {
	s.Lock()
	defer s.Unlock()
	if record, ok := s.records[service.Name()]; ok {
		return record.Source
	}
	s.records[service.Name()] = &entity.SupervisorStats{
		Name:   service.Name(),
		Source: service,
	}
	return nil
}

// snapshot returns a copy, nil if unknown.
func (s *statsStore) snapshot(name string) *entity.SupervisorStats {
	s.RLock()
	defer s.RUnlock()
	stat, ok := s.records[name]
	if !ok {
		return nil
	}
	cp := *stat
	return &cp
}

// update runs fn under the write lock; keep blocking work out of fn.
func (s *statsStore) update(name string, fn func(stat *entity.SupervisorStats)) {
	s.Lock()
	defer s.Unlock()
	if stat, ok := s.records[name]; ok {
		fn(stat)
	}
}
