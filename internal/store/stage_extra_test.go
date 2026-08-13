package store

import (
	"context"
	"fmt"
	"testing"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read fail") }

func TestStageReadError(t *testing.T) {
	st := testStore(t)
	if _, err := st.Stage(context.Background(), errReader{}, "x.tar.gz", 1); err == nil {
		t.Fatal("expected stream error")
	}
}
