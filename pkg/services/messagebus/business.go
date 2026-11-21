package messagebus

import (
	"context"

	"github.com/xhanio/errors"

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
		if err := m.subscribe(ctx, module); err != nil {
			m.log.Error(err)
		}
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

// subscribe replaces svc's subscription, if it has one: its listener ends with
// the channel it reads.
func (m *manager) subscribe(ctx context.Context, svc common.Named) error {
	_, isMH := svc.(common.MessageHandler)
	_, isRMH := svc.(common.RawMessageHandler)
	if !isMH && !isRMH {
		return nil
	}
	m.subMu.Lock()
	defer m.subMu.Unlock()
	if err := m.bus.Unsubscribe(svc.Name(), m.topic); err != nil {
		return errors.Wrapf(err, "failed to unsubscribe %s", svc.Name())
	}
	ch, err := m.bus.Subscribe(svc.Name(), m.topic)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe %s", svc.Name())
	}
	if ch == nil {
		return nil
	}
	m.wg.Go(func() { m.listen(ctx, svc, ch) })
	return nil
}

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
