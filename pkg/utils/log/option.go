package log

import (
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// defaultTimeFormat is the log line timestamp layout.
const defaultTimeFormat = "01/02/2006 15:04:05.00"

type Option func(*logger)

func (l *logger) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(l)
	}
}

func (l *logger) setDefaults() {
	if l.timeFormat == "" {
		l.timeFormat = defaultTimeFormat
	}
}

func WithLevel(level int) Option {
	return func(l *logger) {
		l.level = zapcore.Level(level)
	}
}

func WithTimeFormat(format string) Option {
	return func(l *logger) {
		l.timeFormat = format
	}
}

func WithFileWriter(file string, maxSize, maxBackups, maxAge int) Option {
	return func(l *logger) {
		if file != "" {
			l.fileWriter = &lumberjack.Logger{
				Filename:   file,
				MaxSize:    maxSize,
				MaxBackups: maxBackups,
				MaxAge:     maxAge,
			}
		}
	}
}

// NoStdout suppresses the colored console core so log records are only
// written to the file writer (when configured). Useful for daemons that pipe
// stdout into an external log collector or want silent-by-default binaries.
func NoStdout() Option {
	return func(l *logger) {
		l.noStdout = true
	}
}
