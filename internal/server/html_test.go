package server

import "testing"

func TestHumanSize(t *testing.T) {
	if humanSize(15) != "15 B" {
		t.Fatal(humanSize(15))
	}
	if humanSize(2048) != "2.0 KiB" {
		t.Fatal(humanSize(2048))
	}
}

func TestLoginErrorCopy(t *testing.T) {
	if loginErrorCopy("unauthorized") != "Username or password is wrong." {
		t.Fatal(loginErrorCopy("unauthorized"))
	}
	if loginErrorCopy("") != "" {
		t.Fatal("empty")
	}
}
