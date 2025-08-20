package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/xhanio/framingo/pkg/types/common"
)

var Default = New(WithLevel(-1))

func Level() zapcore.Level                        { return Default.Level() }
func Debug(args ...interface{})                   { Default.Debug(args...) }
func Info(args ...interface{})                    { Default.Info(args...) }
func Warn(args ...interface{})                    { Default.Warn(args...) }
func Error(args ...interface{})                   { Default.Error(args...) }
func Fatal(args ...interface{})                   { Default.Fatal(args...) }
func Debugln(args ...interface{})                 { Default.Debugln(args...) }
func Infoln(args ...interface{})                  { Default.Infoln(args...) }
func Warnln(args ...interface{})                  { Default.Warnln(args...) }
func Errorln(args ...interface{})                 { Default.Errorln(args...) }
func Fatalln(args ...interface{})                 { Default.Fatalln(args...) }
func Debugf(template string, args ...interface{}) { Default.Debugf(template, args...) }
func Infof(template string, args ...interface{})  { Default.Infof(template, args...) }
func Warnf(template string, args ...interface{})  { Default.Warnf(template, args...) }
func Errorf(template string, args ...interface{}) { Default.Errorf(template, args...) }
func Fatalf(template string, args ...interface{}) { Default.Fatalf(template, args...) }
func Sugared() *zap.SugaredLogger                 { return Default.Sugared() }
func With(args ...interface{}) Logger             { return Default.With(args...) }
func By(caller common.Named) Logger               { return Default.By(caller) }
