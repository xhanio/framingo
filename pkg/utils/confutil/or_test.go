package confutil

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// An Or getter takes the key where the config sets it, a zero or a false included,
// and the default where it does not.
func TestOrTakesAKeyTheConfigSets(t *testing.T) {
	v := viper.New()
	v.Set("svc.interval", "2s")
	v.Set("svc.enabled", false)
	v.Set("svc.max", 0)
	c := New(v)

	assert.Equal(t, 2*time.Second, c.GetDurationOr(time.Minute, "%s.interval", "svc"))
	assert.False(t, c.GetBoolOr(true, "%s.enabled", "svc"), "a false the config sets is set")
	assert.Equal(t, 0, c.GetIntOr(10, "%s.max", "svc"), "so is a zero")
	assert.Equal(t, "base", c.GetStringOr("base", "%s.name", "svc"), "a key the config does not set: the default")
	assert.Equal(t, 0.5, c.GetFloat64Or(0.5, "%s.ratio", "svc"))
}

// Every getter has its Or twin, slices, maps and times among them.
func TestOrCoversEveryType(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	v := viper.New()
	v.Set("svc.hosts", []string{"a", "b"})
	v.Set("svc.labels", map[string]any{"env": "prod"})
	v.Set("svc.since", at)
	c := New(v)

	assert.Equal(t, []string{"a", "b"}, c.GetStringSliceOr([]string{"x"}, "svc.hosts"))
	assert.Equal(t, []string{"x"}, c.GetStringSliceOr([]string{"x"}, "svc.none"))
	assert.Equal(t, map[string]any{"env": "prod"}, c.GetStringMapOr(nil, "svc.labels"))
	assert.Equal(t, at, c.GetTimeOr(time.Time{}, "svc.since"))
	assert.Equal(t, uint64(7), c.GetUint64Or(7, "svc.none"))
	assert.Equal(t, "fallback", c.GetOr("fallback", "svc.none"))
}
