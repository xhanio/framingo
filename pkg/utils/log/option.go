package log

import (
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Option func(*logger)

func WithLevel(level int) Option {
	return func(l *logger) {
		l.level = zapcore.Level(level)
	}
}

func WithTimeFormat(format string) Option {
	return func(l *logger) {
		if format != "" {
			l.timeFormat = format
		}
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
