package log

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Sugared() *zap.SugaredLogger
	Level() zapcore.Level

	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Fatal(args ...any)
	Debugln(args ...any)
	Infoln(args ...any)
	Warnln(args ...any)
	Errorln(args ...any)
	Fatalln(args ...any)
	Debugf(template string, args ...any)
	Infof(template string, args ...any)
	Warnf(template string, args ...any)
	Errorf(template string, args ...any)
	Fatalf(template string, args ...any)

	With(args ...any) Logger
	By(caller common.Named) Logger
}
