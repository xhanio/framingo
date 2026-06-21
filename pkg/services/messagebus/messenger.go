package messagebus

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
)

var _ model.Messenger = (*messenger)(nil)

type messenger struct {
	name  string
	topic string
	bus   model.Pubsub
	ch    <-chan entity.PubsubMessage
}

func (m *manager) NewMessenger(name string) (model.Messenger, error) {
	if name == "" {
		return nil, errors.Newf("messenger name cannot be empty")
	}
	ch, err := m.bus.Subscribe(name, m.topic)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe messenger %s", name)
	}
	return &messenger{
		name:  name,
		topic: m.topic,
		bus:   m.bus,
		ch:    ch,
	}, nil
}

func (m *messenger) Name() string                   { return m.name }
func (m *messenger) Dependencies() []common.Service { return nil }

func (m *messenger) Ch() <-chan entity.PubsubMessage { return m.ch }

func (m *messenger) Send(ctx context.Context, kind string, payload any) error {
	return m.bus.Publish(ctx, m.name, m.topic, kind, payload)
}

func (m *messenger) Close() {
	_ = m.bus.Unsubscribe(m.name, m.topic)
}
