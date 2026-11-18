package nameutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/types/info"
)

type sampleService struct{}

func TestPackage(t *testing.T) {
	assert.Equal(t, "nameutil", Package(&sampleService{}))
	assert.Equal(t, "nameutil", Package(sampleService{}))
}

func TestName(t *testing.T) {
	info.ProjectPath = "xhanio/framingo"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}))
	assert.Equal(t, "utils/nameutil/sampleService", Name(sampleService{}))
}

func TestNameRootOverride(t *testing.T) {
	info.ProjectPath = "xhanio/framingo"
	t.Cleanup(func() {
		info.ProjectPath = ""
		Root = "pkg"
	})
	Root = ""
	assert.Equal(t, "pkg/utils/nameutil/sampleService", Name(&sampleService{}))
	Root = "pkg/utils"
	assert.Equal(t, "nameutil/sampleService", Name(&sampleService{}))
}

func TestNameWithoutProjectPath(t *testing.T) {
	assert.Equal(t, "github.com/xhanio/framingo/pkg/utils/nameutil/sampleService", Name(&sampleService{}))
}

func TestNameNil(t *testing.T) {
	assert.Empty(t, Name(nil))
}

func TestNameStripsGivenPrefix(t *testing.T) {
	info.ProjectPath = "xhanio/framingo"
	t.Cleanup(func() { info.ProjectPath = "" })

	assert.Equal(t, "nameutil/sampleService", Name(&sampleService{}, "utils"))
	assert.Equal(t, "sampleService", Name(&sampleService{}, "utils/nameutil"), "prefix covering the whole dir leaves the type name")
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}, "services"), "unrelated prefix strips nothing")
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}, "util"), "partial segment strips nothing")
	assert.Equal(t, "nameutil/sampleService", Name(&sampleService{}, "services", "utils"), "first matching prefix wins")
}

func TestRelativeTrimsProjectPathOverlap(t *testing.T) {
	info.ProjectPath = "foo/myapp"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "internal/services/db", relative("github.com/foo/myapp/internal/services/db"))
	assert.Equal(t, "", relative("github.com/foo/myapp"))
}

func TestRelativeTrimsPartialOverlapForDependencies(t *testing.T) {
	info.ProjectPath = "xhanio/framingo/example"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "pkg/services/db", relative("github.com/xhanio/framingo/pkg/services/db"))
}

func TestRelativeOverlapBoundaries(t *testing.T) {
	t.Cleanup(func() { info.ProjectPath = "" })

	info.ProjectPath = "foo/my"
	assert.Equal(t, "github.com/foo/myapp/x", relative("github.com/foo/myapp/x"), "partial segment")

	info.ProjectPath = "xhanio/framingo/example"
	assert.Equal(t, "github.com/xhanio/gopro/pkg/utils", relative("github.com/xhanio/gopro/pkg/utils"), "org-only overlap")

	info.ProjectPath = "myapp"
	assert.Equal(t, "services/db", relative("github.com/org/myapp/services/db"), "single-segment project")
}

func TestRelativeWithoutProjectPath(t *testing.T) {
	assert.Equal(t, "github.com/xhanio/framingo/pkg/services/db", relative("github.com/xhanio/framingo/pkg/services/db"))
}

func TestConfigRoot(t *testing.T) {
	assert.Equal(t, "db", ConfigRoot("services/db/manager"))
	assert.Equal(t, "db", ConfigRoot("db/manager"))
	assert.Equal(t, "db", ConfigRoot("github.com/xhanio/framingo/pkg/services/db/manager"))
	assert.Equal(t, "clickhouse", ConfigRoot("clickhouse"))
	assert.Equal(t, "", ConfigRoot(""))
}
