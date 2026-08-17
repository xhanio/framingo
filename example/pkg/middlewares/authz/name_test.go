package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "authz", New(nil).Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "authz2", New(nil, WithName("authz2")).Name())
	// an empty override keeps the default
	assert.Equal(t, "authz", New(nil, WithName("")).Name())
}
