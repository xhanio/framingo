package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/services/tracing/exporters/model"
)

func TestRetryOnIsNilSafe(t *testing.T) {
	// exported vocabulary must be total: a factory calling cfg.Retry.On()
	// unguarded gets the default (on), not a panic
	var r *model.Retry
	assert.True(t, r.On())
}

func TestRetryOnTriState(t *testing.T) {
	assert.True(t, (&model.Retry{}).On())
	on, off := true, false
	assert.True(t, (&model.Retry{Enabled: &on}).On())
	assert.False(t, (&model.Retry{Enabled: &off}).On())
}
