package confutil

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// A format with no args is the key itself - every plain-string call site
// keeps working, and a stray % in such a key stays literal.
func TestPlainKey(t *testing.T) {
	v := viper.New()
	v.Set("db.connection.max_open", 7)
	v.Set("odd%key", "kept")
	c := New(v)
	assert.Equal(t, 7, c.GetInt("db.connection.max_open"))
	assert.Equal(t, "kept", c.GetString("odd%key"))
}

// With args the format is Sprintf-ed first, so a service reads under a root
// it chooses at runtime - the second database's keys, not the package's.
func TestFormattedKey(t *testing.T) {
	v := viper.New()
	v.Set("clickhouse.connection.max_open", 3)
	v.Set("clickhouse.connection.exec_timeout", "5s")
	c := New(v)
	assert.Equal(t, 3, c.GetInt("%s.connection.max_open", "clickhouse"))
	assert.Equal(t, 5*time.Second, c.GetDuration("%s.connection.%s", "clickhouse", "exec_timeout"))
	assert.Equal(t, 0, c.GetInt("%s.connection.max_open", "postgres"))
}

func TestIsSet(t *testing.T) {
	v := viper.New()
	v.Set("tracing.enabled", false)
	c := New(v)
	assert.True(t, c.IsSet("tracing.enabled"))
	assert.True(t, c.IsSet("%s.enabled", "tracing"))
	assert.False(t, c.IsSet("metrics.enabled"))
}

// A nil viper wraps an empty one: usable, every getter zero.
func TestNewNil(t *testing.T) {
	c := New(nil)
	assert.Equal(t, "", c.GetString("anything"))
	assert.False(t, c.IsSet("anything"))
}

// FromContext never returns nil: a bare context yields an empty Config, so
// every getter reads zero and no caller needs a nil check.
func TestFromContextBare(t *testing.T) {
	c := FromContext(context.Background())
	assert.NotNil(t, c)
	assert.Equal(t, 0, c.GetInt("db.connection.max_open"))
}

// WrapContext carries the very Config FromContext hands back.
func TestContextRoundTrip(t *testing.T) {
	v := viper.New()
	v.Set("example.greeting", "hi")
	c := New(v)
	ctx := WrapContext(context.Background(), c)
	assert.Same(t, c, FromContext(ctx))
	assert.Equal(t, "hi", FromContext(ctx).GetString("%s.greeting", "example"))
}
