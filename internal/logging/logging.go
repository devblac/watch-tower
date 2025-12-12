package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a minimal structured logger with secret redaction.
func New() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
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
	return strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "key") || strings.Contains(k, "pass")
}

