package infra

import (
	"github.com/xhanio/framingo/pkg/types/model"
)

type manager struct {
	db         model.Database
	meter      model.Meter
	pubsub     model.Pubsub
	messagebus model.MessageBus
}

var _ Infra = (*manager)(nil)

// New combines already-constructed services; building them stays in the
// wiring layer.
func New(db model.Database, meter model.Meter, pubsub model.Pubsub, messagebus model.MessageBus) Infra {
	return &manager{
		db:         db,
		meter:      meter,
		pubsub:     pubsub,
		messagebus: messagebus,
	}
}

func (m *manager) DB() model.Database {
	return m.db
}

func (m *manager) Meter() model.Meter {
	return m.meter
}

func (m *manager) PubSub() model.Pubsub {
	return m.pubsub
}

func (m *manager) MessageBus() model.MessageBus {
	return m.messagebus
}
