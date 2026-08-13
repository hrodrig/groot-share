package store

import (
	"context"
	"strings"
	"testing"
)

func TestInsertListAudit(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if err := st.InsertAudit(ctx, Audit{
		Actor: "root", ActorID: 1, Action: "upload",
		ObjectID: "abc", ObjectKey: "run.tar.gz", RemoteIP: "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListAudit(ctx, 10)
	if err != nil || len(got) != 1 || got[0].Action != "upload" || got[0].Actor != "root" {
		t.Fatalf("%+v %v", got, err)
	}
	blob, err := dumpAudit(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(blob, "correct-horse") || strings.Contains(blob, "gfs_") {
		t.Fatalf("secrets in audit: %s", blob)
	}
}

func dumpAudit(st *Store) (string, error) {
	var s string
	err := st.db.QueryRow(`SELECT ifnull(group_concat(actor||action||object_id||object_key||remote_ip), '') FROM audit`).Scan(&s)
	return s, err
}
