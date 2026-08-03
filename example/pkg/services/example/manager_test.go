package example

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/types/info"
)

// With the gopro-injected ProjectPath in place, service names are
// project-relative - and identically so whether the package belongs to the
// application or to framingo as a dependency, since names key supervisor
// stats and the messagebus registry. Without the injection names keep the
// full import path.
func TestServiceNamesAreProjectRelative(t *testing.T) {
	info.ProjectPath = "xhanio/framingo/example"
	t.Cleanup(func() { info.ProjectPath = "" })

	svc := newManager(nil, nil)
	assert.Equal(t, "services/example/manager", svc.Name())

	sv := supervisor.New(nil)
	assert.Equal(t, "services/supervisor/manager", sv.Name())
}
