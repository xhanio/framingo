package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "feature", New().Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "feature2", New(WithName("feature2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "feature", New(WithName("")).Name())
}
