package reflectutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type locateSample struct{}

func TestLocateReturnsImportPath(t *testing.T) {
	pkg, name := Locate(&locateSample{})
	assert.Equal(t, "github.com/xhanio/framingo/pkg/utils/reflectutil", pkg)
	assert.Equal(t, "locateSample", name)
}

func TestLocateNil(t *testing.T) {
	pkg, name := Locate(nil)
	assert.Empty(t, pkg)
	assert.Empty(t, name)
}
