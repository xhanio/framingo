package pubsub

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// SendMessage implements common.MessageSender.
// The topic is derived from the sender's name.
func (m *manager) SendMessage(ctx context.Context, from common.Named, message common.Message) {
	if message == nil || from == nil {
		return
	}
	m.Publish(from, from.Name(), message.Kind(), message)
}

// SendRawMessage implements common.RawMessageSender.
func (m *manager) SendRawMessage(ctx context.Context, from common.Named, kind string, payload any) {
	if from == nil {
		return
	}
	m.Publish(from, from.Name(), kind, payload)
}

func (m *manager) Publish(from common.Named, topic string, kind string, payload any) {
	m.published.Add(1)

	name := ""
	if from != nil {
		name = from.Name()
	}
	if err := m.bus.Publish(name, topic, kind, payload); err != nil {
		m.log.Errorf("failed to publish to backend: topic=%s error=%v", topic, err)
	}
}

func (m *manager) Subscribe(svc common.Named, topic string) {
	if svc == nil {
		return
	}
	ch, err := m.bus.Subscribe(svc.Name(), topic)
	if err != nil {
		m.log.Errorf("failed to subscribe: topic=%s service=%s error=%v", topic, svc.Name(), err)
		return
	}
	if ch == nil {
		return
	}
	m.wg.Add(1)
	go m.listen(svc, ch)
}

// listen reads messages from a subscription channel and dispatches to MessageHandler/RawMessageHandler.
func (m *manager) listen(svc common.Named, ch <-chan entity.PubsubMessage) {
	defer m.wg.Done()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if mh, isMessageHandler := svc.(common.MessageHandler); isMessageHandler {
				if e, ok := msg.Payload.(common.Message); ok {
					if err := mh.HandleMessage(m.ctx, e); err != nil {
						m.log.Errorf("error handling message: subscriber=%s error=%v", svc.Name(), err)
					}
				}
			}
			if rmh, isRawMessageHandler := svc.(common.RawMessageHandler); isRawMessageHandler {
				if err := rmh.HandleRawMessage(m.ctx, msg.Kind, msg.Payload); err != nil {
					m.log.Errorf("error handling raw message: subscriber=%s error=%v", svc.Name(), err)
				}
			}
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *manager) Unsubscribe(svc common.Named, topic string) {
	if svc == nil {
		return
	}
	if err := m.bus.Unsubscribe(svc.Name(), topic); err != nil {
		m.log.Errorf("failed to unsubscribe: topic=%s service=%s error=%v", topic, svc.Name(), err)
	}
}
