package logger

import (
	"context"
	"github.com/natefinch/lumberjack"
	"github.com/your-username/forum/internal/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var Lg *zap.Logger

func LgWithContext(ctx context.Context) *zap.Logger {
	if traceID, ok := ctx.Value(middleware.TraceIDKey).(string); ok {
		return Lg.With(zap.String("traceID", traceID))
	}
	return Lg
}

func Init(mode string) (err error) {
	writeSyncer := getLogWriter("logs/app.log", 10, 5, 30)
	encoder := getEncoder()
	var l = new(zapcore.Level)
	err = l.UnmarshalText([]byte("info"))
	if err != nil {
		return
	}
	var core zapcore.Core
	if mode == "dev" {
		consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, writeSyncer, l),
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel),
		)
	} else {
		core = zapcore.NewCore(encoder, writeSyncer, l)
	}

	Lg = zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(Lg)
	return
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(filename string, maxSize, maxBackup, maxAge int) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackup,
		MaxAge:     maxAge,
	}
	return zapcore.AddSync(lumberJackLogger)
}
