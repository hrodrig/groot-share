// Package logging configures process-wide slog.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup configures the default slog logger. format: json|text; level: debug|info|warn|error.
// SPEC §8: JSON lines to stdout (parseable by jq / Vector / Fluent Bit).
func Setup(format, level string) {
	SetupWriter(os.Stdout, format, level)
}

// SetupWriter is like Setup but writes to w (tests).
func SetupWriter(w io.Writer, format, level string) {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if strings.EqualFold(format, "text") {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	slog.SetDefault(slog.New(h))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
