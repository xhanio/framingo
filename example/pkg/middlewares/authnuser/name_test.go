package authnuser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "authnuser", New(nil).Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "authnuser2", New(nil, WithName("authnuser2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "authnuser", New(nil, WithName("")).Name())
}
