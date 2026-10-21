//go:build windows

// Command wtfsync-tray is wtfsync's tray app: a tray icon showing sync state, a menu to push and
// pull, and windows for setup, conflicts and settings. Build with -ldflags "-H windowsgui".
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/applog"
	"github.com/rhwendt/wtfsync/internal/autostart"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/ui"
	"github.com/rhwendt/wtfsync/internal/update"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		// No console in a GUI program: the log file is where this ends up.
		fmt.Fprintln(os.Stderr, "wtfsync-tray:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := os.UserConfigDir() // %APPDATA%
	if err != nil {
		return err
	}
	dataDir := filepath.Join(cfg, "wtfsync")
	logFile, err := applog.Open(filepath.Join(dataDir, "tray.log"), applog.DefaultMax)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)
	// A GUI program has nowhere to print a crash: send it, with every goroutine's stack, to a file.
	if crash, err := os.OpenFile(filepath.Join(dataDir, "crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		defer crash.Close()
		debug.SetTraceback("all")
		if err := debug.SetCrashOutput(crash, debug.CrashOptions{}); err != nil {
			logger.Printf("crash log: %v", err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logger.Printf("wtfsync-tray %s starting", version)
	err = ui.Run(app.Deps{
		Store:     local.Store{Dir: dataDir},
		Procs:     wow.System(),
		Hydrator:  syncdir.TimeoutHydrator{Timeout: syncdir.DefaultHydrateTimeout},
		Now:       time.Now,
		Updates:   update.GitHub(),
		Autostart: autostart.Run{Name: "wtfsync", Command: `"` + exe + `"`},
		Version:   version,
	}, ui.Options{InstallDirs: wow.DefaultInstallDirs(), Getenv: os.Getenv, Hostname: os.Hostname, Log: logger, LogDir: dataDir})
	if err != nil {
		logger.Printf("exit: %v", err)
	}
	return err
}
