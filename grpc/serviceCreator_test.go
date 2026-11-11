package grpc

import (
	"bytes"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLoggerDefaultsToWarn(t *testing.T) {
	core := newLogger(&Settings{}).Core()

	if core.Enabled(zap.InfoLevel) {
		t.Fatal("expected info to be disabled by default")
	}
	if !core.Enabled(zap.WarnLevel) {
		t.Fatal("expected warn to be enabled by default")
	}
}

func TestNewLoggerUsesConfiguredLevel(t *testing.T) {
	core := newLogger(&Settings{LogLevel: "error"}).Core()

	if core.Enabled(zap.WarnLevel) {
		t.Fatal("expected warn to be disabled at error level")
	}
	if !core.Enabled(zap.ErrorLevel) {
		t.Fatal("expected error to be enabled at error level")
	}
}

func TestNewLoggerPanicsOnInvalidLevel(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid log level to panic")
		}
	}()

	newLogger(&Settings{LogLevel: "verbose"})
}

func TestNewLoggerDisablesStacktrace(t *testing.T) {
	for _, disable := range []bool{false, true} {
		var buf bytes.Buffer
		logger := newLogger(&Settings{DisableStacktrace: disable})
		logger = logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), c)
		}))
		logger.Error("boom")

		hasStack := bytes.Contains(buf.Bytes(), []byte(`"stacktrace"`))
		if hasStack == disable {
			t.Fatalf("DisableStacktrace=%v but stacktrace present=%v: %s", disable, hasStack, buf.String())
		}
	}
}
