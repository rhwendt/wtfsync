// Command wtfsync syncs World of Warcraft addon settings between PCs through a cloud-synced folder.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rhwendt/wtfsync/internal/cli"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	cfg, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtfsync:", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.Deps{
		Procs:          wow.System(),
		Hydrator:       syncdir.TimeoutHydrator{Timeout: syncdir.DefaultHydrateTimeout},
		Now:            time.Now,
		Hostname:       os.Hostname,
		Getenv:         os.Getenv,
		DefaultDataDir: filepath.Join(cfg, "wtfsync"),
		InstallDirs:    wow.DefaultInstallDirs(),
		Version:        version,
	}))
}
