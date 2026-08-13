package store

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestStageTransitRetryCleanup(t *testing.T) {
	st := testStore(t)
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
