package server

import (
	"net/url"
	"strings"
	"testing"
)

func TestHumanSize(t *testing.T) {
	if humanSize(15) != "15 B" {
		t.Fatal(humanSize(15))
	}
	if humanSize(2048) != "2.0 KiB" {
		t.Fatal(humanSize(2048))
	}
}

func TestLoginErrorCopy(t *testing.T) {
	if loginErrorCopy("unauthorized") != "Incorrect username or password." {
		t.Fatal(loginErrorCopy("unauthorized"))
	}
	if loginErrorCopy("") != "" {
		t.Fatal("empty")
	}
}

func TestDisplayVersion(t *testing.T) {
	if displayVersion("") != "dev" {
		t.Fatal(displayVersion(""))
	}
	if displayVersion("0.1.0") != "0.1.0" {
		t.Fatal(displayVersion("0.1.0"))
	}
}

func TestNoticeFromQueryUploadedName(t *testing.T) {
	q := url.Values{}
	q.Set("notice", "uploaded")
	q.Set("name", "run.tar.gz")
	kind, text := noticeFromQuery(q)
	if kind != "ok" || text != "Capture run.tar.gz uploaded. You can send another." {
		t.Fatalf("%q %q", kind, text)
	}
	q.Set("name", "../../../etc/passwd")
	kind, text = noticeFromQuery(q)
	if kind != "ok" || text != "Capture passwd uploaded. You can send another." {
		t.Fatalf("basename name: %q %q", kind, text)
	}
	q.Set("name", "my file.tar.gz")
	kind, text = noticeFromQuery(q)
	if text != "Capture my-file.tar.gz uploaded. You can send another." {
		t.Fatalf("spaces: %q", text)
	}
	q.Set("name", "bad\u0000name")
	kind, text = noticeFromQuery(q)
	if text != "Capture badname uploaded. You can send another." {
		t.Fatalf("control char: %q", text)
	}
}

func TestNoticeFromQueryDuplicateName(t *testing.T) {
	q := url.Values{}
	q.Set("notice", "duplicate")
	q.Set("name", "run.tar.gz")
	kind, text := noticeFromQuery(q)
	if kind != "err" || text != "Capture run.tar.gz is already uploaded (same content). Check Captures or pick another file." {
		t.Fatalf("%q %q", kind, text)
	}
	q.Set("name", "my file.tar.gz")
	kind, text = noticeFromQuery(q)
	if text != "Capture my-file.tar.gz is already uploaded (same content). Check Captures or pick another file." {
		t.Fatalf("spaces: %q", text)
	}
	q.Del("name")
	kind, text = noticeFromQuery(q)
	if text != "This file is already uploaded (same content). Check Captures or pick another file." {
		t.Fatalf("no name: %q", text)
	}
}

func TestLoginPageShell(t *testing.T) {
	if !strings.Contains(themeHeadScript, "gfs-theme") {
		t.Fatal("theme script missing key")
	}
	if !strings.Contains(passwordToggleScript, "login-password") {
		t.Fatal("password toggle missing input id")
	}
}
