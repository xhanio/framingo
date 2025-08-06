package log

import (
	"io"

	"go.uber.org/zap/zapcore"
)

type Option func(*logger)

func WithLevel(level int) Option {
	return func(l *logger) {
		l.level = zapcore.Level(level)
	}
}

func WithFileWriter(w io.Writer) Option {
	return func(l *logger) {
		l.fileWriter = w
	}
}
