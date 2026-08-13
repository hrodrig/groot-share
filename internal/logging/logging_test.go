package logging_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/logging"
)

func TestSetupWriterJSONPrefixed(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "debug")
	slog.Info("hello")
	out := buf.String()
	if !strings.HasPrefix(out, "gfs ") {
		t.Fatalf("prefix missing: %q", out)
	}
	if !strings.Contains(out, `"msg":"hello"`) && !strings.Contains(out, "hello") {
		t.Fatalf("msg missing: %q", out)
	}
}

func TestSetupWriterTextLevels(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "text", "warn")
	slog.Info("hidden")
	slog.Warn("shown")
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatal("info should be filtered")
	}
	if !strings.Contains(out, "shown") {
		t.Fatalf("warn missing: %q", out)
	}
}

func TestSetupWriterPartialLines(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	slog.Info("one")
	if !strings.Contains(buf.String(), "gfs ") {
		t.Fatal(buf.String())
	}
}

func TestSetupDefault(t *testing.T) {
	logging.Setup("text", "error")
	slog.Error("boom")
}

func TestPrefixWriterChunks(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	slog.Info("line-a")
	slog.Info("line-b")
	out := buf.String()
	if strings.Count(out, "gfs ") < 2 {
		t.Fatalf("want 2 prefixes: %q", out)
	}
}
