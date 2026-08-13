// Package main is the gfs entrypoint.
package main

import (
	"fmt"
	"os"
)

// Set via -ldflags at build time (Makefile / GoReleaser).
var (
	version   = "dev"
	commit    = "unknown"
	branch    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-V":
			fmt.Printf("gfs %s commit=%s branch=%s date=%s\n", version, commit, branch, buildDate)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "gfs %s — HTTP server is Phase 2 (see docs/SPECIFICATIONS.md)\n", version)
	return 0
}
