package zapctx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	Info(ctx, "hello")
	assert.Equal(t, "INFO\thello\n", buf.String())
}

func TestLoggerPanicNilContext(t *testing.T) {
	assert.PanicsWithValue(t, "nil context passed to zapctx.Logger()",
		func() { Logger(nil) }) //nolint:staticcheck // for test
}

func TestLoggerPanicNilLogger(t *testing.T) {
	assert.PanicsWithValue(t, "context without logger passed to zapctx.Logger()",
		func() {
			ctx := WithLogger(t.Context(), nil)
			Logger(ctx)
		})
}

func TestLoggerPanicNoLogger(t *testing.T) {
	assert.PanicsWithValue(t, "context without logger passed to zapctx.Logger()",
		func() {
			ctx := t.Context()
			Logger(ctx)
		})
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	outputLogger := Logger(ctx)
	assert.Equal(t, logger, outputLogger)
}

func TestWithFields0(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	ctx = WithFields(ctx)
	Info(ctx, "hello")
	assert.Equal(t, "INFO\thello\n", buf.String())
}

func TestWithFields1(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	ctx = WithFields(ctx, zap.Int("foo", 999), zap.String("bar", "abc_abc"))
	Info(ctx, "hello")
	assert.Equal(t, "INFO\thello\t{\"foo\": 999, \"bar\": \"abc_abc\"}\n", buf.String())
}

func TestWithFields2(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	Info(ctx, "hello", zap.Int("foo", 999), zap.String("bar", "abc_abc"))
	assert.Equal(t, "INFO\thello\t{\"foo\": 999, \"bar\": \"abc_abc\"}\n", buf.String())
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.DebugLevel)
	ctx := WithLogger(t.Context(), logger)
	messageAllLevels(ctx)
	assert.Equal(t, "DEBUG\thello\nINFO\thello\nWARN\thello\nERROR\thello\n", buf.String())
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.InfoLevel)
	ctx := WithLogger(t.Context(), logger)
	messageAllLevels(ctx)
	assert.Equal(t, "INFO\thello\nWARN\thello\nERROR\thello\n", buf.String())
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.WarnLevel)
	ctx := WithLogger(t.Context(), logger)
	messageAllLevels(ctx)
	assert.Equal(t, "WARN\thello\nERROR\thello\n", buf.String())
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, zapcore.ErrorLevel)
	ctx := WithLogger(t.Context(), logger)
	messageAllLevels(ctx)
	assert.Equal(t, "ERROR\thello\n", buf.String())
}

func TestLoggerForCaller(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core, zap.AddCaller())
	ctx := WithLogger(t.Context(), logger)
	Debug(ctx, "Hello_debug_wrap")
	Logger(ctx).Debug("Hello_debug_direct")
	Info(ctx, "Hello_info_wrap")
	Logger(ctx).Info("Hello_info_direct")
	Warn(ctx, "Hello_warn_wrap")
	Logger(ctx).Warn("Hello_warn_direct")
	Error(ctx, "Hello_error_wrap")
	Logger(ctx).Error("Hello_error_direct")

	actual := make([]string, len(logs.All()))
	for i, entry := range logs.All() {
		actual[i] = loggedEntryToString(entry)
	}
	expected := []string{
		"zapctx.TestLoggerForCaller Hello_debug_wrap debug",
		"zapctx.TestLoggerForCaller Hello_debug_direct debug",
		"zapctx.TestLoggerForCaller Hello_info_wrap info",
		"zapctx.TestLoggerForCaller Hello_info_direct info",
		"zapctx.TestLoggerForCaller Hello_warn_wrap warn",
		"zapctx.TestLoggerForCaller Hello_warn_direct warn",
		"zapctx.TestLoggerForCaller Hello_error_wrap error",
		"zapctx.TestLoggerForCaller Hello_error_direct error",
	}
	assert.Equal(t, expected, actual)
}

func loggedEntryToString(entry observer.LoggedEntry) string {
	caller := entry.Caller.Function
	caller = caller[strings.LastIndex(caller, "/")+1:]
	return strings.TrimSpace(fmt.Sprintln(caller, entry.Message, entry.Level))
}

func messageAllLevels(ctx context.Context) {
	Debug(ctx, "hello")
	Info(ctx, "hello")
	Warn(ctx, "hello")
	Error(ctx, "hello")
}

func newLogger(w io.Writer, level zapcore.Level) *zap.Logger {
	config := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.CapitalLevelEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.AddSync(w),
		level,
	)
	return zap.New(core)
}
