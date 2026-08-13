package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestTransitBySHA256Empty(t *testing.T) {
	st := testStore(t)
	if _, err := st.TransitBySHA256(context.Background(), ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty: %v", err)
	}
}

func TestDeleteTransitNotFound(t *testing.T) {
	st := testStore(t)
	if err := st.DeleteTransit(context.Background(), "00000000000000000000000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestTransitBySHA256RoundTrip(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("hash-me")), "t.tar.gz", 0)
	if err != nil {
		t.Fatal(err)
	}
	s3key := "captures/t/" + staged.ID + ".tar.gz"
	if err := st.SaveTransit(ctx, staged, s3key, "err"); err != nil {
		t.Fatal(err)
	}
	tr, err := st.TransitBySHA256(ctx, staged.SHA256)
	if err != nil || tr.S3Key != s3key {
		t.Fatalf("%+v %v", tr, err)
	}
}
