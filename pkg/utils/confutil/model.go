package confutil

import "time"

// Config is the read side of the app's configuration, handed to services
// through the Init context. Every getter takes a format string: with no
// args it is the key itself, otherwise it is fmt.Sprintf-ed first, so a
// service reads under its own root - GetInt("%s.connection.max_open",
// nameutil.ConfigRoot(m.name)) - rather than a hardcoded one. Getters mirror viper's: an
// absent key reads as the zero value, never an error.
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
}
