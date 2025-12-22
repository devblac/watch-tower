package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a minimal structured logger with secret redaction.
func New() *slog.Logger {
	return NewWithLevel("info")
}

// NewWithLevel creates a logger with the specified level (debug, info, warn, error).
func NewWithLevel(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if isSecretKey(a.Key) {
				a.Value = slog.StringValue("[redacted]")
			}
			return a
		},
	})
	return slog.New(handler)
}

func isSecretKey(k string) bool {
	k = strings.ToLower(k)
	return strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "key") || strings.Contains(k, "pass") || strings.Contains(k, "password")
}
