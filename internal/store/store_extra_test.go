package store

import (
	"context"
	"testing"
)

func TestCloseTwice(t *testing.T) {
	st := testStore(t)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if st.Ping(context.Background()) {
		t.Fatal("ping after close")
	}
}

func TestListAuditPagination(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := st.InsertAudit(ctx, Audit{
			Actor: "root", Action: "upload", ObjectID: "id", ObjectKey: "k",
		}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := st.ListAuditPage(ctx, 10, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("all %+v %v", all, err)
	}
	page, err := st.ListAuditPage(ctx, 2, 2)
	if err != nil || len(page) != 1 {
		t.Fatalf("last page %+v %v", page, err)
	}
}
