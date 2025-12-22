package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretRedaction(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if isSecretKey(a.Key) {
				a.Value = slog.StringValue("[redacted]")
			}
			return a
		},
	})
	logger := slog.New(handler)

	tests := []struct {
		key    string
		value  string
		should bool
	}{
		{"api_token", "secret123", true},
		{"API_KEY", "key456", true},
		{"password", "pass789", true},
		{"secret", "mysecret", true},
		{"webhook_url", "https://example.com", false},
		{"message", "hello", false},
		{"count", "42", false},
	}

	for _, tt := range tests {
		buf.Reset()
		logger.Info("test", tt.key, tt.value)
		output := buf.String()

		if tt.should {
			if !strings.Contains(output, "[redacted]") {
				t.Errorf("key %q should be redacted, output: %s", tt.key, output)
			}
			if strings.Contains(output, tt.value) {
				t.Errorf("key %q value %q should not appear, output: %s", tt.key, tt.value, output)
			}
		} else {
			if strings.Contains(output, "[redacted]") {
				t.Errorf("key %q should not be redacted, output: %s", tt.key, output)
			}
			if !strings.Contains(output, tt.value) {
				t.Errorf("key %q value %q should appear, output: %s", tt.key, tt.value, output)
			}
		}
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"invalid", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		logger := NewWithLevel(tt.level)
		if logger == nil {
			t.Errorf("NewWithLevel(%q) returned nil", tt.level)
		}
	}
}
