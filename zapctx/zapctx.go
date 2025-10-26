// (see github.com/uber-go/zap) with contexts.
package zapctx

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// loggerKey holds the context key used for loggers.
type loggerKey struct{}

// WithLogger returns a new context derived from ctx that
// is associated with the given logger.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// WithFields returns a new context derived from ctx
// that has a logger that always logs the given fields.
func WithFields(ctx context.Context, fields ...zapcore.Field) context.Context {
	return WithLogger(ctx, Logger(ctx).With(fields...))
}

// Logger returns the logger associated with the given
// context. If there is no context or no logger, it will panic.
func Logger(ctx context.Context) *zap.Logger {
	if ctx == nil {
		panic("nil context passed to zapctx.Logger()")
	}
	logger, _ := ctx.Value(loggerKey{}).(*zap.Logger)
	if logger == nil {
		panic("context without logger passed to zapctx.Logger()")
	}
	return logger
}

func Debug(ctx context.Context, msg string, fields ...zapcore.Field) {
	loggerForCaller(ctx).Debug(msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zapcore.Field) {
	loggerForCaller(ctx).Info(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zapcore.Field) {
	loggerForCaller(ctx).Warn(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zapcore.Field) {
	loggerForCaller(ctx).Error(msg, fields...)
}

func loggerForCaller(ctx context.Context) *zap.Logger {
	return Logger(ctx).WithOptions(zap.AddCaller(), zap.AddCallerSkip(1))
}
