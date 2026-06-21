package messagebus

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type target struct {
	typed atomic.Int64
	raw   atomic.Int64
}

func (t *target) Name() string                   { return "msg_target" }
func (t *target) Dependencies() []common.Service { return nil }

func (t *target) HandleMessage(ctx context.Context, m common.Message) error {
	if _, ok := m.(*msg); ok {
		t.typed.Add(1)
	}
	return nil
}

func (t *target) HandleRawMessage(ctx context.Context, kind string, payload any) error {
	t.raw.Add(1)
	return nil
}

type source struct{}

func (s *source) Name() string                   { return "msg_source" }
func (s *source) Dependencies() []common.Service { return nil }

type msg struct{ Body string }

func (m *msg) Kind() string { return "test_message" }

func waitFor(t *testing.T, deadline time.Duration, cond func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func newBus(t *testing.T) pubsub.Manager {
	t.Helper()
	bus := pubsub.New(driver.NewMemory(log.Default))
	require.NoError(t, bus.Start(context.Background()))
	t.Cleanup(func() { _ = bus.Stop(true) })
	return bus
}

func newMessageBus(t *testing.T, opts ...Option) Manager {
	t.Helper()
	mb := New(newBus(t), opts...)
	require.NoError(t, mb.Start(context.Background()))
	t.Cleanup(func() { _ = mb.Stop(true) })
	return mb
}

func TestSendMessage_DispatchesToMessageHandler(t *testing.T) {
	mb := newMessageBus(t)

	tt := &target{}
	mb.Register(tt)
	ss := &source{}

	const count = 50
	for i := 0; i < count; i++ {
		mb.SendMessage(context.Background(), ss, &msg{Body: "hi"})
	}

	waitFor(t, 2*time.Second, func() bool { return tt.typed.Load() == count })
	if got := tt.raw.Load(); got != count {
		t.Fatalf("expected %d raw deliveries (RawMessageHandler is catch-all), got %d", count, got)
	}
}

func TestSendRawMessage_DispatchesToRawHandler(t *testing.T) {
	mb := newMessageBus(t)

	tt := &target{}
	mb.Register(tt)
	ss := &source{}

	const count = 20
	for i := 0; i < count; i++ {
		mb.SendRawMessage(context.Background(), ss, "raw_kind", map[string]int{"i": i})
	}

	waitFor(t, 2*time.Second, func() bool { return tt.raw.Load() == count })
	if got := tt.typed.Load(); got != 0 {
		t.Fatalf("raw payload should not reach MessageHandler, got %d typed deliveries", got)
	}
}

func TestSenderDoesNotReceiveOwnMessage(t *testing.T) {
	mb := newMessageBus(t)

	tt := &target{}
	mb.Register(tt)

	mb.SendMessage(context.Background(), tt, &msg{Body: "self"})

	time.Sleep(100 * time.Millisecond)
	if got := tt.typed.Load(); got != 0 {
		t.Fatalf("sender should not receive own message, got %d typed deliveries", got)
	}
}

func TestWithTopic_IsolatesBuses(t *testing.T) {
	bus := newBus(t)
	a := New(bus, WithTopic("/a"), WithName("messagebus_a"))
	b := New(bus, WithTopic("/b"), WithName("messagebus_b"))
	require.NoError(t, a.Start(context.Background()))
	require.NoError(t, b.Start(context.Background()))
	t.Cleanup(func() { _ = a.Stop(true); _ = b.Stop(true) })

	ta := &target{}
	tb := &target{}
	a.Register(ta)
	b.Register(tb)
	ss := &source{}

	a.SendMessage(context.Background(), ss, &msg{Body: "to_a"})

	waitFor(t, 1*time.Second, func() bool { return ta.typed.Load() == 1 })
	time.Sleep(50 * time.Millisecond)
	if got := tb.typed.Load(); got != 0 {
		t.Fatalf("message on /a should not reach subscriber on /b, got %d", got)
	}
}

func TestRegister_AfterStart_SubscribesImmediately(t *testing.T) {
	mb := newMessageBus(t)

	tt := &target{}
	mb.Register(tt) // after Start
	ss := &source{}

	mb.SendMessage(context.Background(), ss, &msg{Body: "hi"})
	waitFor(t, time.Second, func() bool { return tt.typed.Load() == 1 })
}

func TestNewMessenger_ReceivesAndSends(t *testing.T) {
	mb := newMessageBus(t)

	m1, err := mb.NewMessenger("client-1")
	require.NoError(t, err)
	defer m1.Close()

	m2, err := mb.NewMessenger("client-2")
	require.NoError(t, err)
	defer m2.Close()

	// m1 sends; m2 receives; m1 does NOT receive its own message.
	require.NoError(t, m1.Send(context.Background(), "greet", "hello"))

	select {
	case got := <-m2.Ch():
		assert.Equal(t, "client-1", got.From)
		assert.Equal(t, "greet", got.Kind)
		assert.Equal(t, "hello", got.Payload)
	case <-time.After(time.Second):
		t.Fatal("m2 did not receive message")
	}

	select {
	case got := <-m1.Ch():
		t.Fatalf("m1 received own message: %v", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestNewMessenger_EmptyNameRejected(t *testing.T) {
	mb := newMessageBus(t)
	_, err := mb.NewMessenger("")
	assert.Error(t, err)
}

func TestNewMessenger_CloseClosesChannel(t *testing.T) {
	mb := newMessageBus(t)
	m1, err := mb.NewMessenger("client")
	require.NoError(t, err)
	m1.Close()
	_, ok := <-m1.Ch()
	assert.False(t, ok, "channel should be closed after Close")
}

func TestNewMessenger_JSONRoundTrip(t *testing.T) {
	// Verifies the wire format used by AttachWebSocket.
	original := entity.PubsubMessage{From: "a", Topic: "/t", Kind: "k", Payload: "p"}
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"from":"a"`)
	assert.Contains(t, string(data), `"topic":"/t"`)
	assert.Contains(t, string(data), `"kind":"k"`)
	assert.Contains(t, string(data), `"payload":"p"`)

	var decoded entity.PubsubMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}
