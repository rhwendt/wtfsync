// Package cli implements the wtfsync command line: setup (init, link) and push, pull, status, restore.
package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// Deps are the outside-world pieces Run needs: main passes real ones, tests pass fakes.
type Deps struct {
	Procs          wow.Processes
	Hydrator       syncdir.Hydrator
	Now            func() time.Time
	Hostname       func() (string, error)
	Getenv         func(string) string
	DefaultDataDir string   // %APPDATA%\wtfsync
	InstallDirs    []string // likely WoW install roots, most likely first
}

const (
	exitOK      = 0
	exitErr     = 1
	exitRefused = 2 // game running, or snapshot not downloaded yet
	exitAborted = 3 // a conflict prompt was aborted
)

const usage = `usage: wtfsync <command> [flags]

commands:
  init      set up the first PC: pick the sync folder, then push
  link      set up another PC on the same sync folder, then pull
  push      upload this PC's addon settings
  pull      download the newest addon settings from your other PCs
  status    show sync state for each flavor
  restore   undo the last pull from its backup (--set <id> for an older one)

Run init or link again to change this PC's WoW folder, sync folder or name.

flags:
  --flavor <name>     only this flavor (default: all enabled)
  --dry-run           show what would change; change nothing
  --wow-dir <path>    WoW install folder (overrides settings)
  --sync-dir <path>   cloud-synced folder (overrides settings)
  --data-dir <path>   wtfsync's local data folder (default %APPDATA%\wtfsync)
  --name <name>       this PC's display name (init, link)
  --set <id>          backup set to restore (restore)
`

type cmd struct {
	deps Deps
	in   *bufio.Reader
	out  io.Writer
	errw io.Writer
}

// Run runs wtfsync with args (without the program name) and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, deps Deps) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitErr
	}
	c := &cmd{deps: deps, in: bufio.NewReader(stdin), out: stdout, errw: stderr}
	var err error
	switch args[0] {
	case "init":
		err = c.setup(args[1:], true)
	case "link":
		err = c.setup(args[1:], false)
	case "push", "pull", "status", "restore":
		err = c.sync(args[0], args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		err = fmt.Errorf("unknown command %q (see `wtfsync help`)", args[0])
	}
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stderr, "wtfsync:", err)
	}
	return exitCode(err)
}

func exitCode(err error) int {
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.Is(err, engine.ErrAborted):
		return exitAborted
	case errors.Is(err, engine.ErrGameRunning), errors.Is(err, syncdir.ErrNotHydrated):
		return exitRefused
	default:
		return exitErr
	}
}

type flags struct {
	flavor, wowDir, syncDir, dataDir, name, set string
	dryRun                                      bool
}

func (c *cmd) parse(name string, args []string) (flags, error) {
	var f flags
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(c.errw)
	set.Usage = func() { fmt.Fprint(c.errw, usage) }
	set.StringVar(&f.flavor, "flavor", "", "")
	set.BoolVar(&f.dryRun, "dry-run", false, "")
	set.StringVar(&f.wowDir, "wow-dir", "", "")
	set.StringVar(&f.syncDir, "sync-dir", "", "")
	set.StringVar(&f.dataDir, "data-dir", c.deps.DefaultDataDir, "")
	set.StringVar(&f.name, "name", "", "")
	set.StringVar(&f.set, "set", "", "")
	if err := set.Parse(args); err != nil {
		return f, err
	}
	if set.NArg() > 0 {
		return f, fmt.Errorf("unexpected argument %q", set.Arg(0))
	}
	return f, nil
}

func (c *cmd) prompter() prompter { return prompter{in: c.in, out: c.out} }

func (c *cmd) engine(store local.Store, s local.Settings) *engine.Engine {
	return &engine.Engine{
		Store:    store,
		Settings: s,
		Procs:    c.deps.Procs,
		Hydrator: c.deps.Hydrator,
		Prompt:   c.prompter(),
		Now:      c.deps.Now,
	}
}
