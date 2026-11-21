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

// New accepts a nil viper, yielding a Config whose getters all read zero.
func New(v *viper.Viper) Config {
	if v == nil {
		v = viper.New()
	}
	return &config{v: v}
}

// key skips Sprintf without args, so a literal % in a key survives.
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

func (c *config) GetOr(def any, format string, args ...any) any {
	return or(c, c.Get, def, format, args...)
}

func (c *config) GetBoolOr(def bool, format string, args ...any) bool {
	return or(c, c.GetBool, def, format, args...)
}

func (c *config) GetDurationOr(def time.Duration, format string, args ...any) time.Duration {
	return or(c, c.GetDuration, def, format, args...)
}

func (c *config) GetFloat64Or(def float64, format string, args ...any) float64 {
	return or(c, c.GetFloat64, def, format, args...)
}

func (c *config) GetIntOr(def int, format string, args ...any) int {
	return or(c, c.GetInt, def, format, args...)
}

func (c *config) GetInt32Or(def int32, format string, args ...any) int32 {
	return or(c, c.GetInt32, def, format, args...)
}

func (c *config) GetInt64Or(def int64, format string, args ...any) int64 {
	return or(c, c.GetInt64, def, format, args...)
}

func (c *config) GetIntSliceOr(def []int, format string, args ...any) []int {
	return or(c, c.GetIntSlice, def, format, args...)
}

func (c *config) GetSizeInBytesOr(def uint, format string, args ...any) uint {
	return or(c, c.GetSizeInBytes, def, format, args...)
}

func (c *config) GetStringOr(def string, format string, args ...any) string {
	return or(c, c.GetString, def, format, args...)
}

func (c *config) GetStringMapOr(def map[string]any, format string, args ...any) map[string]any {
	return or(c, c.GetStringMap, def, format, args...)
}

func (c *config) GetStringMapStringOr(def map[string]string, format string, args ...any) map[string]string {
	return or(c, c.GetStringMapString, def, format, args...)
}

func (c *config) GetStringMapStringSliceOr(def map[string][]string, format string, args ...any) map[string][]string {
	return or(c, c.GetStringMapStringSlice, def, format, args...)
}

func (c *config) GetStringSliceOr(def []string, format string, args ...any) []string {
	return or(c, c.GetStringSlice, def, format, args...)
}

func (c *config) GetTimeOr(def time.Time, format string, args ...any) time.Time {
	return or(c, c.GetTime, def, format, args...)
}

func (c *config) GetUintOr(def uint, format string, args ...any) uint {
	return or(c, c.GetUint, def, format, args...)
}

func (c *config) GetUint8Or(def uint8, format string, args ...any) uint8 {
	return or(c, c.GetUint8, def, format, args...)
}

func (c *config) GetUint16Or(def uint16, format string, args ...any) uint16 {
	return or(c, c.GetUint16, def, format, args...)
}

func (c *config) GetUint32Or(def uint32, format string, args ...any) uint32 {
	return or(c, c.GetUint32, def, format, args...)
}

func (c *config) GetUint64Or(def uint64, format string, args ...any) uint64 {
	return or(c, c.GetUint64, def, format, args...)
}

// or is get's value at the key where c sets it, def where it does not.
func or[T any](c *config, get func(string, ...any) T, def T, format string, args ...any) T {
	if !c.IsSet(format, args...) {
		return def
	}
	return get(format, args...)
}
