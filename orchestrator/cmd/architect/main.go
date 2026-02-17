// Package main provides an installable binary entry point for the orchestrator.
// This is an alternative to the top-level main.go for `go install` usage.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Find and exec the orchestrator binary from the same directory.
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Dir(exe)
	orchestrator := filepath.Join(dir, "orchestrator")

	cmd := exec.Command(orchestrator, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
