package confutil

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type config struct {
	v *viper.Viper
}

var _ Config = (*config)(nil)

// New wraps a viper instance. A nil viper wraps an empty one, so the result
// is always usable and every getter yields the zero value - the same
// never-nil doctrine FromContext follows.
func New(v *viper.Viper) Config {
	if v == nil {
		v = viper.New()
	}
	return &config{v: v}
}

// key is the one place a format becomes a key: untouched without args, so a
// literal key never meets Sprintf and a stray % in it stays literal.
func key(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

func (c *config) Get(format string, args ...any) any { return c.v.Get(key(format, args...)) }

func (c *config) GetBool(format string, args ...any) bool { return c.v.GetBool(key(format, args...)) }

func (c *config) GetDuration(format string, args ...any) time.Duration {
	return c.v.GetDuration(key(format, args...))
}

func (c *config) GetFloat64(format string, args ...any) float64 {
	return c.v.GetFloat64(key(format, args...))
}

func (c *config) GetInt(format string, args ...any) int { return c.v.GetInt(key(format, args...)) }

func (c *config) GetInt32(format string, args ...any) int32 {
	return c.v.GetInt32(key(format, args...))
}

func (c *config) GetInt64(format string, args ...any) int64 {
	return c.v.GetInt64(key(format, args...))
}

func (c *config) GetIntSlice(format string, args ...any) []int {
	return c.v.GetIntSlice(key(format, args...))
}

func (c *config) GetSizeInBytes(format string, args ...any) uint {
	return c.v.GetSizeInBytes(key(format, args...))
}

func (c *config) GetString(format string, args ...any) string {
	return c.v.GetString(key(format, args...))
}

func (c *config) GetStringMap(format string, args ...any) map[string]any {
	return c.v.GetStringMap(key(format, args...))
}

func (c *config) GetStringMapString(format string, args ...any) map[string]string {
	return c.v.GetStringMapString(key(format, args...))
}

func (c *config) GetStringMapStringSlice(format string, args ...any) map[string][]string {
	return c.v.GetStringMapStringSlice(key(format, args...))
}

func (c *config) GetStringSlice(format string, args ...any) []string {
	return c.v.GetStringSlice(key(format, args...))
}

func (c *config) GetTime(format string, args ...any) time.Time {
	return c.v.GetTime(key(format, args...))
}

func (c *config) GetUint(format string, args ...any) uint { return c.v.GetUint(key(format, args...)) }

func (c *config) GetUint8(format string, args ...any) uint8 {
	return c.v.GetUint8(key(format, args...))
}

func (c *config) GetUint16(format string, args ...any) uint16 {
	return c.v.GetUint16(key(format, args...))
}

func (c *config) GetUint32(format string, args ...any) uint32 {
	return c.v.GetUint32(key(format, args...))
}

func (c *config) GetUint64(format string, args ...any) uint64 {
	return c.v.GetUint64(key(format, args...))
}

func (c *config) IsSet(format string, args ...any) bool { return c.v.IsSet(key(format, args...)) }
