package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
)

func dropInboxTar(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWatchOnceIngestsStableFile(t *testing.T) {
	s, st := identServer(t)
	inbox := t.TempDir()
	s.Cfg.SFTPInbox = inbox
	seen := map[string]int64{}
	p := dropInboxTar(t, inbox, "groot-run.tar.gz", "sftp-payload-vps")

	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("first poll must not ingest")
	}

	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("inbox file should be gone")
	}
	list, err := st.ListArchives(context.Background())
	if err != nil || len(list) != 1 || list[0].Source != "sftp" || list[0].Key != "groot-run.tar.gz" {
		t.Fatalf("archives %+v %v", list, err)
	}
	events, err := st.ListAudit(context.Background(), 10)
	if err != nil || len(events) == 0 || events[0].Actor != "sftp" || events[0].Action != "upload" {
		t.Fatalf("audit %+v %v", events, err)
	}

	ck := loginCookie(t, s)
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "pill-sftp") {
		t.Fatalf("html %d body missing pill-sftp", rr.Code)
	}
}

func TestWatchOnceDuplicateRemovesInbox(t *testing.T) {
	s, _ := identServer(t)
	inbox := t.TempDir()
	s.Cfg.SFTPInbox = inbox
	seen := map[string]int64{}
	body := "same-sftp-bytes"
	dropInboxTar(t, inbox, "one.tar.gz", body)
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	p2 := dropInboxTar(t, inbox, "two.tar.gz", body)
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p2); !os.IsNotExist(err) {
		t.Fatal("duplicate should be removed from inbox")
	}
	list, err := s.Store.ListArchives(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("want 1 archive got %+v %v", list, err)
	}
}

func TestWatchOnceVPSS3UsesSFTPKey(t *testing.T) {
	s, mem := vpsS3Server(t)
	inbox := t.TempDir()
	s.Cfg.SFTPInbox = inbox
	seen := map[string]int64{}
	dropInboxTar(t, inbox, "cluster-run.tar.gz", "sftp-s3-bytes")
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	objs, err := mem.List(context.Background(), "captures/")
	if err != nil || len(objs) != 1 {
		t.Fatalf("objects %+v %v", objs, err)
	}
	if blob.SourceForKey(objs[0].Key) != "sftp" || !strings.Contains(objs[0].Key, "/sftp/") {
		t.Fatalf("key %s", objs[0].Key)
	}
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Items []struct {
			Source string `json:"source"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || len(out.Items) != 1 || out.Items[0].Source != "sftp" {
		t.Fatalf("json %+v %v", out, err)
	}
}

func TestWatchOnceSkipsDotfilesAndUnset(t *testing.T) {
	s, _ := identServer(t)
	if err := s.WatchOnce(context.Background(), map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	inbox := t.TempDir()
	s.Cfg.SFTPInbox = inbox
	dropInboxTar(t, inbox, ".hidden.tar.gz", "nope")
	if err := os.WriteFile(filepath.Join(inbox, "notes.txt"), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int64{}
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	if err := s.WatchOnce(context.Background(), seen); err != nil {
		t.Fatal(err)
	}
	list, err := s.Store.ListArchives(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("got %+v %v", list, err)
	}
}

func TestWatchOnceMissingDir(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.SFTPInbox = filepath.Join(t.TempDir(), "missing")
	if err := s.WatchOnce(context.Background(), map[string]int64{}); err != nil {
		t.Fatal(err)
	}
}

func TestWatchLoopUnsetReturns(t *testing.T) {
	s, _ := identServer(t)
	done := make(chan struct{})
	go func() {
		s.WatchLoop(context.Background(), time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WatchLoop must return when inbox unset")
	}
}

func TestWatchOnceNilSeen(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.SFTPInbox = t.TempDir()
	if err := s.WatchOnce(context.Background(), nil); err == nil {
		t.Fatal("expected nil map error")
	}
}

func TestWatchLoopStopsOnCancel(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.SFTPInbox = t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.WatchLoop(ctx, 5*time.Millisecond)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WatchLoop did not stop")
	}
}
