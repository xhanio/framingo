package example

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/info"
	fmodel "github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/repo"
)

func TestServiceNamesAreProjectRelative(t *testing.T) {
	info.ProjectPath = "xhanio/framingo/example"
	t.Cleanup(func() { info.ProjectPath = "" })

	svc := newManager(platform.New(nil, nil, nil, nil), nil)
	assert.Equal(t, "services/example/manager", svc.Name())

	sv := supervisor.New(nil)
	assert.Equal(t, "services/supervisor/manager", sv.Name())
}

// unimplemented methods hit the embedded nil interface and panic
type fakeRepo struct {
	repo.HelloWorld
}

func (fakeRepo) CreateHelloWorld(_ context.Context, message string) (*entity.HelloWorld, error) {
	return &entity.HelloWorld{ID: 1, Message: message}, nil
}

type fakeMB struct {
	fmodel.MessageBus
}

func (fakeMB) SendRawMessage(context.Context, common.Named, string, any) {}

func TestHelloWorldIncrementsCounter(t *testing.T) {
	reader, provider := testutil.NewMeterRecorder()
	m := newManager(platform.New(nil, testutil.NewMeter(provider), nil, fakeMB{}), fakeRepo{})
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

// with no WithName the config root is the package name
func TestInitReadsGreetingUnderRoot(t *testing.T) {
	_, provider := testutil.NewMeterRecorder()
	m := newManager(platform.New(nil, testutil.NewMeter(provider), nil, fakeMB{}), fakeRepo{})
	v := viper.New()
	v.Set("example.greeting", "hi there")
	require.NoError(t, m.Init(confutil.WrapContext(context.Background(), confutil.New(v))))
	assert.Equal(t, "hi there", m.greeting)
}

func TestHelloWorldWithoutMeter(t *testing.T) {
	m := newManager(platform.New(nil, nil, nil, fakeMB{}), fakeRepo{})
	require.NoError(t, m.Init(context.Background()))

	//nolint:staticcheck // SA1029: intentional, the api context keys are shared with echo's string-keyed store
	ctx := context.WithValue(context.Background(), api.ContextKeyCredential, &entity.Credential{UserName: "tester"})
	_, err := m.HelloWorld(ctx, "hi")
	require.NoError(t, err)
}

func TestHelloWorldWithoutActor(t *testing.T) {
	m := newManager(platform.New(nil, nil, nil, fakeMB{}), fakeRepo{})
	require.NoError(t, m.Init(context.Background()))

	_, err := m.HelloWorld(context.Background(), "hi")
	assert.True(t, errors.Is(err, errors.Unauthorized), "a ctx with no credential: got %v", err)
}
