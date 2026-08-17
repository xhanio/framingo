package throttle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "throttle", New().Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "throttle2", New(WithName("throttle2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "throttle", New(WithName("")).Name())
}
