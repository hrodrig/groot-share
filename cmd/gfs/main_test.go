package main

import "testing"

func TestRunVersion(t *testing.T) {
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("code %d", code)
	}
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("--version %d", code)
	}
	if code := run([]string{"-V"}); code != 0 {
		t.Fatalf("-V %d", code)
	}
}

func TestRunBare(t *testing.T) {
	if code := run(nil); code != 0 {
		t.Fatalf("bare invoke %d", code)
	}
}
