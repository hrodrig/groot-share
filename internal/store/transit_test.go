package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestStageTransitRetryCleanup(t *testing.T) {
	st := archiveStore(t)
	ctx := context.Background()
	staged, err := st.Stage(ctx, bytes.NewReader([]byte("pending")), "run.tar.gz", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staged.Path); err != nil {
		t.Fatal(err)
	}
	s3key := "captures/2026/08/12/" + staged.ID + ".tar.gz"
	if err := st.SaveTransit(ctx, staged, s3key, "put failed"); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListTransit(ctx)
	if err != nil || len(list) != 1 || list[0].S3Key != s3key {
		t.Fatalf("%+v %v", list, err)
	}
	got, err := st.TransitByS3Key(ctx, s3key)
	if err != nil || got.Key != "run.tar.gz" {
		t.Fatalf("%+v %v", got, err)
	}
	if err := st.UpdateTransitError(ctx, staged.ID, "still failing"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTransit(ctx, staged.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staged.Path); !os.IsNotExist(err) {
		t.Fatalf("staging should be gone: %v", err)
	}
	empty, err := st.ListTransit(ctx)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty %+v %v", empty, err)
	}
}

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
