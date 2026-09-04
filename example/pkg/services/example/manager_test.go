package example

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/info"
	fmodel "github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

// testPlatform fakes the service's narrow Platform view - the payoff of
// consumer side interfaces: two methods instead of the app's whole bundle.
type testPlatform struct {
	meter fmodel.Meter
	mb    fmodel.MessageBus
}

func (t testPlatform) Meter() fmodel.Meter           { return t.meter }
func (t testPlatform) MessageBus() fmodel.MessageBus { return t.mb }

// With the gopro-injected ProjectPath in place, service names are
// project-relative - and identically so whether the package belongs to the
// application or to framingo as a dependency, since names key supervisor
// stats and the messagebus registry. Without the injection names keep the
// full import path.
func TestServiceNamesAreProjectRelative(t *testing.T) {
	info.ProjectPath = "xhanio/framingo/example"
	t.Cleanup(func() { info.ProjectPath = "" })

	svc := newManager(testPlatform{}, nil)
	assert.Equal(t, "services/example/manager", svc.Name())

	sv := supervisor.New(nil)
	assert.Equal(t, "services/supervisor/manager", sv.Name())
}

// fakeRepo embeds the nil interface so only the one method HelloWorld
// touches needs a body - any other call is a test bug and panics.
type fakeRepo struct {
	repository.Repository
}

func (fakeRepo) CreateHelloWorld(_ context.Context, message string) (*orm.HelloWorld, error) {
	return &orm.HelloWorld{ID: 1, Message: message}, nil
}

// fakeMB embeds the nil interface the same way: SendRawMessage is the one
// method HelloWorld drives.
type fakeMB struct {
	fmodel.MessageBus
}

func (fakeMB) SendRawMessage(context.Context, common.Named, string, any) {}

func TestHelloWorldIncrementsCounter(t *testing.T) {
	reader, provider := testutil.NewMeterRecorder()
	m := newManager(testPlatform{meter: testutil.NewMeter(provider), mb: fakeMB{}}, fakeRepo{})
	require.NoError(t, m.Init(context.Background()))

	//nolint:staticcheck // SA1029: intentional, the api context keys are shared with echo's string-keyed store
	ctx := context.WithValue(context.Background(), api.ContextKeyCredential, &entity.Credential{UserName: "tester"})
	_, err := m.HelloWorld(ctx, "hi")
	require.NoError(t, err)
	_, err = m.HelloWorld(ctx, "again")
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	total, found := testutil.SumInt64(rm, "example.hello.total")
	require.True(t, found, "example.hello.total missing from the collected metrics")
	assert.Equal(t, int64(2), total)
}

// Init reads the greeting under the service's config root - its package
// name, example, since the service carries no WithName.
func TestInitReadsGreetingUnderRoot(t *testing.T) {
	_, provider := testutil.NewMeterRecorder()
	m := newManager(testPlatform{meter: testutil.NewMeter(provider), mb: fakeMB{}}, fakeRepo{})
	v := viper.New()
	v.Set("example.greeting", "hi there")
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	assert.Equal(t, "hi there", m.greeting)
}
