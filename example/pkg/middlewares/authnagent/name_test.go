package authnagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "authnagent", New(nil).Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "authnagent2", New(nil, WithName("authnagent2")).Name())
	assert.Equal(t, "authnagent", New(nil, WithName("")).Name())
}
