//go:build !windows

// Command wtfsync-tray is wtfsync's Windows tray app; on other systems it only says so.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "wtfsync-tray runs on Windows; use the wtfsync command here.")
	os.Exit(1)
}
