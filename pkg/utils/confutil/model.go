package confutil

import "time"

// Config keys are Sprintf-ed only when args are given; an absent key reads zero, never an error.
type Config interface {
	Get(format string, args ...any) any
	GetBool(format string, args ...any) bool
	GetDuration(format string, args ...any) time.Duration
	GetFloat64(format string, args ...any) float64
	GetInt(format string, args ...any) int
	GetInt32(format string, args ...any) int32
	GetInt64(format string, args ...any) int64
	GetIntSlice(format string, args ...any) []int
	GetSizeInBytes(format string, args ...any) uint
	GetString(format string, args ...any) string
	GetStringMap(format string, args ...any) map[string]any
	GetStringMapString(format string, args ...any) map[string]string
	GetStringMapStringSlice(format string, args ...any) map[string][]string
	GetStringSlice(format string, args ...any) []string
	GetTime(format string, args ...any) time.Time
	GetUint(format string, args ...any) uint
	GetUint8(format string, args ...any) uint8
	GetUint16(format string, args ...any) uint16
	GetUint32(format string, args ...any) uint32
	GetUint64(format string, args ...any) uint64
	IsSet(format string, args ...any) bool

	// The Or getters return the key's value where the config sets it, and def where it
	// does not: a reloadable setting read over its default (design/config.md).
	GetOr(def any, format string, args ...any) any
	GetBoolOr(def bool, format string, args ...any) bool
	GetDurationOr(def time.Duration, format string, args ...any) time.Duration
	GetFloat64Or(def float64, format string, args ...any) float64
	GetIntOr(def int, format string, args ...any) int
	GetInt32Or(def int32, format string, args ...any) int32
	GetInt64Or(def int64, format string, args ...any) int64
	GetIntSliceOr(def []int, format string, args ...any) []int
	GetSizeInBytesOr(def uint, format string, args ...any) uint
	GetStringOr(def string, format string, args ...any) string
	GetStringMapOr(def map[string]any, format string, args ...any) map[string]any
	GetStringMapStringOr(def map[string]string, format string, args ...any) map[string]string
	GetStringMapStringSliceOr(def map[string][]string, format string, args ...any) map[string][]string
	GetStringSliceOr(def []string, format string, args ...any) []string
	GetTimeOr(def time.Time, format string, args ...any) time.Time
	GetUintOr(def uint, format string, args ...any) uint
	GetUint8Or(def uint8, format string, args ...any) uint8
	GetUint16Or(def uint16, format string, args ...any) uint16
	GetUint32Or(def uint32, format string, args ...any) uint32
	GetUint64Or(def uint64, format string, args ...any) uint64
}
