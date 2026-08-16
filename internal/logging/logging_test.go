package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/logging"
)

func TestSetupWriterJSONLines(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "debug")
	slog.Info("hello")
	out := buf.String()
	if strings.HasPrefix(out, "gfs ") {
		t.Fatalf("must not prefix lines: %q", out)
	}
	line := strings.TrimSpace(out)
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("want parseable JSON: %v %q", err, out)
	}
	if payload["msg"] != "hello" {
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
	if !strings.Contains(buf.String(), `"msg":"one"`) {
		t.Fatal(buf.String())
	}
}

func TestSetupDefault(t *testing.T) {
	logging.Setup("text", "error")
	slog.Error("boom")
}

func TestJSONMultipleLines(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	slog.Info("line-a")
	slog.Info("line-b")
	out := buf.String()
	if strings.Contains(out, "gfs ") {
		t.Fatalf("prefix must be gone: %q", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("want 2 lines: %q", out)
	}
	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
	}
}
