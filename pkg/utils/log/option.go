package log

import (
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

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

// Construction options: fixed for the instance's life, set at New (design/config.md).

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

// NoStdout drops the console core; records go only to the file writer, if any.
func NoStdout() Option {
	return func(l *logger) {
		l.noStdout = true
	}
}
