package platform

import (
	fmodel "github.com/xhanio/framingo/pkg/types/model"

	"github.com/xhanio/framingo/example/pkg/types/model"
)

type manager struct {
	db         fmodel.Database
	meter      fmodel.Meter
	pubsub     fmodel.Pubsub
	messagebus fmodel.MessageBus
}

var _ model.Platform = (*manager)(nil)

// New bundles the handles into the model.Platform services take; none may be nil.
func New(db fmodel.Database, meter fmodel.Meter, pubsub fmodel.Pubsub, messagebus fmodel.MessageBus) model.Platform {
	return &manager{
		db:         db,
		meter:      meter,
		pubsub:     pubsub,
		messagebus: messagebus,
	}
}

func (m *manager) DB() fmodel.Database {
	return m.db
}

func (m *manager) Meter() fmodel.Meter {
	return m.meter
}

func (m *manager) PubSub() fmodel.Pubsub {
	return m.pubsub
}

func (m *manager) MessageBus() fmodel.MessageBus {
	return m.messagebus
}
