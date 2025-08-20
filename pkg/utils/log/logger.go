package log

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/pathutil"
)

type logger struct {
	level      zapcore.Level
	timeFormat string
	fileWriter io.Writer

	core *zap.SugaredLogger
}

func New(opts ...Option) Logger {
	return newLogger(opts...)
}

func newLogger(opts ...Option) *logger {
	l := &logger{
		timeFormat: "01/02/2006 15:04:05.00",
	}
	for _, opt := range opts {
		opt(l)
	}
	var zopts []zap.Option
	if zapcore.Level(l.level) == zapcore.DebugLevel {
		zopts = append(zopts, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	l.core = zap.New(zapcore.NewTee(l.newCores(l.fileWriter)...), zopts...).Sugar()
	return l
}

func (l *logger) newCores(w io.Writer) []zapcore.Core {
	jsonEncoder := zap.NewProductionEncoderConfig()
	jsonEncoder.EncodeTime = zapcore.ISO8601TimeEncoder
	var cores []zapcore.Core
	if w != nil {
		jsonCore := zapcore.NewCore(zapcore.NewJSONEncoder(jsonEncoder), zapcore.AddSync(w), l.level)
		cores = append(cores, jsonCore)
	}
	consoleEncoder := zap.NewProductionEncoderConfig()
	if l.level == zapcore.DebugLevel {
		consoleEncoder = zap.NewDevelopmentEncoderConfig()
	}
	consoleEncoder.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder.EncodeTime = zapcore.TimeEncoderOfLayout(l.timeFormat)
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(consoleEncoder), zapcore.AddSync(os.Stdout), l.level)
	cores = append(cores, consoleCore)
	return cores
}

func (l *logger) With(args ...interface{}) Logger {
	c := l.core.With(args...)
	return &logger{
		core: c,
	}
}

func (l *logger) By(caller common.Named) Logger {
	return l.With(zap.String("caller", pathutil.Short(caller.Name())))
}

func (l *logger) Sugared() *zap.SugaredLogger                 { return l.core }
func (l *logger) Level() zapcore.Level                        { return l.level }
func (l *logger) Debug(args ...interface{})                   { l.core.Debug(args...) }
func (l *logger) Info(args ...interface{})                    { l.core.Info(args...) }
func (l *logger) Warn(args ...interface{})                    { l.core.Warn(args...) }
func (l *logger) Error(args ...interface{})                   { l.core.Error(args...) }
func (l *logger) Fatal(args ...interface{})                   { l.core.Fatal(args...) }
func (l *logger) Debugln(args ...interface{})                 { l.core.Debugln(args...) }
func (l *logger) Infoln(args ...interface{})                  { l.core.Infoln(args...) }
func (l *logger) Warnln(args ...interface{})                  { l.core.Warnln(args...) }
func (l *logger) Errorln(args ...interface{})                 { l.core.Errorln(args...) }
func (l *logger) Fatalln(args ...interface{})                 { l.core.Fatalln(args...) }
func (l *logger) Debugf(template string, args ...interface{}) { l.core.Debugf(template, args...) }
func (l *logger) Infof(template string, args ...interface{})  { l.core.Infof(template, args...) }
func (l *logger) Warnf(template string, args ...interface{})  { l.core.Warnf(template, args...) }
func (l *logger) Errorf(template string, args ...interface{}) { l.core.Errorf(template, args...) }
func (l *logger) Fatalf(template string, args ...interface{}) { l.core.Fatalf(template, args...) }
