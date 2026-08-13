package server

import (
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

func TestLoginPageShell(t *testing.T) {
	if !strings.Contains(themeHeadScript, "gfs-theme") {
		t.Fatal("theme script missing key")
	}
	if !strings.Contains(passwordToggleScript, "login-password") {
		t.Fatal("password toggle missing input id")
	}
}
