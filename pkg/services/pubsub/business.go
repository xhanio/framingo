package pubsub

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/entity"
)

func (m *manager) Publish(ctx context.Context, from, topic, kind string, payload any) error {
	m.published.Add(1)
	if err := m.bus.Publish(ctx, from, topic, kind, payload); err != nil {
		m.log.Errorf("failed to publish to backend: topic=%s error=%v", topic, err)
		return err
	}
	return nil
}

func (m *manager) Subscribe(name, topic string) (<-chan entity.PubsubMessage, error) {
	return m.bus.Subscribe(name, topic)
}

func (m *manager) Unsubscribe(name, topic string) error {
	return m.bus.Unsubscribe(name, topic)
}
