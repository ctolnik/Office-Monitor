//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Error: This agent is only supported on Windows")
	os.Exit(1)
}
