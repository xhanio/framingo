package cors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "cors", New().Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "cors2", New(WithName("cors2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "cors", New(WithName("")).Name())
}
