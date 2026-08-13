// Package logging configures process-wide slog (gghstats/trigger-style).
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

const linePrefix = "gfs "

// Setup configures the default slog logger. format: json|text; level: debug|info|warn|error.
// SPEC §8: JSON to stdout (not trigger's stderr).
func Setup(format, level string) {
	SetupWriter(os.Stdout, format, level)
}

// SetupWriter is like Setup but writes to w (tests).
func SetupWriter(w io.Writer, format, level string) {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if strings.EqualFold(format, "text") {
		h = slog.NewTextHandler(&prefixWriter{w: w}, opts)
	} else {
		h = slog.NewJSONHandler(&prefixWriter{w: w}, opts)
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

type prefixWriter struct {
	w   io.Writer
	acc []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.acc = append(p.acc, b...)
	for {
		idx := -1
		for i, c := range p.acc {
			if c == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			return n, nil
		}
		line := p.acc[:idx+1]
		p.acc = append([]byte{}, p.acc[idx+1:]...)
		if _, err := p.w.Write(append([]byte(linePrefix), line...)); err != nil {
			return n, err
		}
	}
}
