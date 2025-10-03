package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/common"
)

var Default = New(WithLevel(-1))

func Level() zapcore.Level                { return Default.Level() }
func Debug(args ...any)                   { Default.Debug(args...) }
func Info(args ...any)                    { Default.Info(args...) }
func Warn(args ...any)                    { Default.Warn(args...) }
func Error(args ...any)                   { Default.Error(args...) }
func Fatal(args ...any)                   { Default.Fatal(args...) }
func Debugln(args ...any)                 { Default.Debugln(args...) }
func Infoln(args ...any)                  { Default.Infoln(args...) }
func Warnln(args ...any)                  { Default.Warnln(args...) }
func Errorln(args ...any)                 { Default.Errorln(args...) }
func Fatalln(args ...any)                 { Default.Fatalln(args...) }
func Debugf(template string, args ...any) { Default.Debugf(template, args...) }
func Infof(template string, args ...any)  { Default.Infof(template, args...) }
func Warnf(template string, args ...any)  { Default.Warnf(template, args...) }
func Errorf(template string, args ...any) { Default.Errorf(template, args...) }
func Fatalf(template string, args ...any) { Default.Fatalf(template, args...) }
func Sugared() *zap.SugaredLogger         { return Default.Sugared() }
func With(args ...any) Logger             { return Default.With(args...) }
func By(caller common.Named) Logger       { return Default.By(caller) }
