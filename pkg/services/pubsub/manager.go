package pubsub

import (
	"context"
	"fmt"
	"io"
	"path"
	"sync"
	"sync/atomic"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/printutil"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

var _ Manager = (*manager)(nil)

type manager struct {
	name string
	log  log.Logger

	bus driver.Driver

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	published atomic.Uint64
}

func New(b driver.Driver, opts ...Option) Manager {
	return newManager(b, opts...)
}

func newManager(b driver.Driver, opts ...Option) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &manager{
		log:     log.Default,
		wg:      &sync.WaitGroup{},
		bus: b,
		ctx:     ctx,
		cancel:  cancel,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	if m.name == "" {
		m.name = path.Join(reflectutil.Locate(m))
	}
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}

func (m *manager) Init() error {
	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if err := m.bus.Start(ctx); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Stop(wait bool) error {
	m.cancel()
	if err := m.bus.Stop(wait); err != nil {
		return errors.Wrap(err)
	}
	if wait {
		m.wg.Wait()
	}
	return nil
}

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
func (m *manager) listen(svc common.Named, ch <-chan driver.Message) {
	defer m.wg.Done()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if mh, isMessageHandler := svc.(common.MessageHandler); isMessageHandler {
				if e, ok := msg.Payload.(common.Message); ok {
					go func() {
						if err := mh.HandleMessage(m.ctx, e); err != nil {
							m.log.Errorf("error handling message: subscriber=%s error=%v", svc.Name(), err)
						}
					}()
				}
			}
			if rmh, isRawMessageHandler := svc.(common.RawMessageHandler); isRawMessageHandler {
				go func() {
					if err := rmh.HandleRawMessage(m.ctx, msg.Kind, msg.Payload); err != nil {
						m.log.Errorf("error handling raw message: subscriber=%s error=%v", svc.Name(), err)
					}
				}()
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

func (m *manager) Info(w io.Writer, debug bool) {
	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("stat", "value")
	t.Row("backend", fmt.Sprintf("%T", m.bus))
	t.Row("published", m.published.Load())
	t.NewLine()
	t.Flush()
}
