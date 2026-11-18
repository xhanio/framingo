package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryOnIsNilSafe(t *testing.T) {
	var r *Retry
	assert.True(t, r.On())
}

func TestRetryOnTriState(t *testing.T) {
	assert.True(t, (&Retry{}).On())
	on, off := true, false
	assert.True(t, (&Retry{Enabled: &on}).On())
	assert.False(t, (&Retry{Enabled: &off}).On())
}
