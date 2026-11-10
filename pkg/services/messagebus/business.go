package messagebus

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

func (m *manager) Register(module common.Named) {
	if module == nil {
		return
	}
	m.mu.Lock()
	if _, exists := m.modules[module.Name()]; exists {
		m.mu.Unlock()
		return
	}
	m.modules[module.Name()] = module
	started, ctx := m.started, m.ctx
	m.mu.Unlock()

	if started {
		m.subscribe(ctx, module)
	}
}

func (m *manager) SendMessage(ctx context.Context, from common.Named, msg common.Message) {
	if from == nil || msg == nil {
		return
	}
	if err := m.bus.Publish(ctx, from.Name(), m.topic, msg.Kind(), msg); err != nil {
		m.log.Errorf("failed to send message: kind=%s error=%v", msg.Kind(), err)
	}
}

func (m *manager) SendRawMessage(ctx context.Context, from common.Named, kind string, payload any) {
	if from == nil {
		return
	}
	if err := m.bus.Publish(ctx, from.Name(), m.topic, kind, payload); err != nil {
		m.log.Errorf("failed to send raw message: kind=%s error=%v", kind, err)
	}
}

// subscribe wires a registered module's dispatch loop to the bus topic. It
// no-ops for modules that implement neither MessageHandler nor RawMessageHandler
// — subscribing them would just drop every message.
func (m *manager) subscribe(ctx context.Context, svc common.Named) {
	_, isMH := svc.(common.MessageHandler)
	_, isRMH := svc.(common.RawMessageHandler)
	if !isMH && !isRMH {
		return
	}
	ch, err := m.bus.Subscribe(svc.Name(), m.topic)
	if err != nil {
		m.log.Errorf("failed to subscribe %s: %v", svc.Name(), err)
		return
	}
	if ch == nil {
		return
	}
	m.wg.Go(func() { m.listen(ctx, svc, ch) })
}

// listen reads messages from a subscription channel and dispatches to
// MessageHandler / RawMessageHandler implementations on svc.
func (m *manager) listen(ctx context.Context, svc common.Named, ch <-chan entity.PubsubMessage) {
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			mh, isMH := svc.(common.MessageHandler)
			rmh, isRMH := svc.(common.RawMessageHandler)
			handled := false
			if isMH {
				if e, ok := msg.Payload.(common.Message); ok {
					handled = true
					if err := mh.HandleMessage(ctx, e); err != nil {
						m.log.Errorf("error handling message: subscriber=%s error=%v", svc.Name(), err)
					}
				}
			}
			if isRMH {
				handled = true
				if err := rmh.HandleRawMessage(ctx, msg.Kind, msg.Payload); err != nil {
					m.log.Errorf("error handling raw message: subscriber=%s error=%v", svc.Name(), err)
				}
			}
			if !handled {
				m.log.Debugf("unhandled message kind=%s payload-type=%T subscriber=%s", msg.Kind, msg.Payload, svc.Name())
			}
		case <-ctx.Done():
			return
		}
	}
}
