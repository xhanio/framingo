package deflate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "deflate", New().Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "deflate2", New(WithName("deflate2")).Name())
	assert.Equal(t, "deflate", New(WithName("")).Name())
}
