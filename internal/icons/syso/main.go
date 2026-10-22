// Command syso writes rsrc_windows_amd64.syso, the exe's icon resource, into each directory given,
// so go build links it into that main package. build.sh runs it before building the release.
//
//	go run ./internal/icons/syso cmd/wtfsync cmd/wtfsync-tray
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rhwendt/wtfsync/internal/icons"
)

func main() {
	for _, dir := range os.Args[1:] {
		if err := write(filepath.Join(dir, "rsrc_windows_amd64.syso")); err != nil {
			fmt.Fprintln(os.Stderr, "syso:", err)
			os.Exit(1)
		}
	}
}

func write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := icons.Resource(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
