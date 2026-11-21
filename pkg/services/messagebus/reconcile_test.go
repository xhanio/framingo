package messagebus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// pubsub restarted alone under the message bus, as the supervisor restarts
// it: every module is subscribed again, once.
func TestAPubsubRestartUnderTheMessageBus(t *testing.T) {
	bus := pubsub.New(driver.NewMemory(log.Default))
	mb := newManager(bus)
	sup := supervisor.New(nil, supervisor.WithLogger(log.New(log.WithLevel(-1))))
	require.NoError(t, sup.Register(bus, mb))
	require.NoError(t, sup.TopoSort())
	require.NoError(t, sup.Init(context.Background()))
	require.NoError(t, sup.Start(context.Background()))
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	tt := &target{}
	mb.Register(tt)
	send := func(want int64) {
		t.Helper()
		mb.SendMessage(context.Background(), &source{}, &msg{Body: "hi"})
		waitFor(t, 2*time.Second, func() bool { return tt.typed.Load() == want })
	}
	send(1)

	require.NoError(t, sup.RestartService(context.Background(), bus.Name()))
	send(2)

	// once more changes nothing: one subscription, one delivery
	require.NoError(t, mb.Reconcile(context.Background(), bus, common.DependencyRunning))
	send(3)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int64(3), tt.typed.Load())
}
