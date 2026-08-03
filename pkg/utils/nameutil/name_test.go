package nameutil

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/types/info"
)

type sampleService struct{}

// Name is the package dir relative to the project root joined with the type
// name, judged from the gopro-injected ProjectPath. The layout root (Root,
// default pkg) is stripped so the category stays visible.
func TestName(t *testing.T) {
	info.ProjectPath = "xhanio/framingo"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}))
	assert.Equal(t, "utils/nameutil/sampleService", Name(sampleService{}))
}

// Projects with a different layout override or clear Root.
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

// Without the injection (plain go build/test) there is no project to be
// relative to: names keep the full import path.
func TestNameWithoutProjectPath(t *testing.T) {
	assert.Equal(t, "github.com/xhanio/framingo/pkg/utils/nameutil/sampleService", Name(&sampleService{}))
}

func TestNameNil(t *testing.T) {
	assert.Empty(t, Name(nil))
}

// Callers may name extra prefixes to strip after Root - they apply to the
// root-stripped dir, first match wins, whole segments only.
func TestNameStripsGivenPrefix(t *testing.T) {
	info.ProjectPath = "xhanio/framingo"
	t.Cleanup(func() { info.ProjectPath = "" })

	assert.Equal(t, "nameutil/sampleService", Name(&sampleService{}, "utils"))
	assert.Equal(t, "sampleService", Name(&sampleService{}, "utils/nameutil"), "prefix covering the whole dir leaves the type name")
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}, "services"), "unrelated prefix strips nothing")
	assert.Equal(t, "utils/nameutil/sampleService", Name(&sampleService{}, "util"), "partial segment strips nothing")
	assert.Equal(t, "nameutil/sampleService", Name(&sampleService{}, "services", "utils"), "first matching prefix wins")
}

// ProjectPath is relative to $GOPATH/src while import paths carry the module
// host prefix, so the cut happens at their overlap - whatever the project's
// layout below it.
func TestRelativeTrimsProjectPathOverlap(t *testing.T) {
	info.ProjectPath = "foo/myapp"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "internal/services/db", relative("github.com/foo/myapp/internal/services/db"))
	assert.Equal(t, "", relative("github.com/foo/myapp"))
}

// A dependency's packages share only the leading segments of ProjectPath:
// the framework used from within its own repo still gets relative names.
func TestRelativeTrimsPartialOverlapForDependencies(t *testing.T) {
	info.ProjectPath = "xhanio/framingo/example"
	t.Cleanup(func() { info.ProjectPath = "" })
	assert.Equal(t, "pkg/services/db", relative("github.com/xhanio/framingo/pkg/services/db"))
}

// Overlaps are whole segments; a single shared segment counts only when it
// is the entire ProjectPath, so an org name shared with an unrelated
// dependency must not cut it.
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
