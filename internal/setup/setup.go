// Package setup sets a PC up for syncing (init and link), for both the CLI and the tray's wizard.
// Running it again on a PC that's set up keeps the PC's identity; moving it to another WoW folder
// or sync folder clears its sync history so the next sync asks like a first link.
package setup

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// Answers are what setup asks for.
type Answers struct {
	WoWDir     string
	SyncFolder string
	Name       string
	First      bool // init: create the sync data and push; otherwise link: join it and pull
}

// Env is what Run needs from outside.
type Env struct {
	Store    local.Store
	Procs    wow.Processes
	Hydrator syncdir.Hydrator
	Prompt   engine.Prompter
	Now      func() time.Time
}

// FlavorResult is the first push or pull of one flavor.
type FlavorResult struct {
	Flavor string
	Report engine.Report
	Err    error
}

type Result struct {
	Settings local.Settings
	Rerun    bool           // the PC was already set up
	Moved    bool           // a rerun that changed the WoW folder or sync folder
	Synced   []FlavorResult // first push (init) or pull (link); empty when only settings changed
}

// Flavors lists a WoW folder's flavors, or says why it isn't a WoW folder.
func Flavors(wowDir string) ([]wow.Flavor, error) {
	flavors, err := wow.Flavors(wowDir)
	if err != nil {
		return nil, err
	}
	if len(flavors) == 0 {
		return nil, fmt.Errorf("no WoW flavors (folders like _retail_) in %s", wowDir)
	}
	return flavors, nil
}

// FindInstalls returns the folders in dirs that hold at least one WoW flavor, in order.
func FindInstalls(dirs []string) []string {
	var found []string
	for _, d := range dirs {
		if fl, err := wow.Flavors(d); err == nil && len(fl) > 0 {
			found = append(found, d)
		}
	}
	return found
}

// SamePath compares folders the way Windows does: cleaned and ignoring letter case.
func SamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Run saves the answers and does the first push or pull. Errors from individual flavors are in
// Result.Synced and also returned joined, so callers can map them to exit codes.
func Run(env Env, a Answers) (Result, error) {
	var res Result
	prev, err := env.Store.LoadSettings()
	res.Rerun = err == nil
	if err != nil && !errors.Is(err, local.ErrNoSettings) {
		return res, err
	}
	flavors, err := Flavors(a.WoWDir)
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(a.Name) == "" {
		return res, errors.New("this PC needs a name")
	}

	sameFolder := res.Rerun && SamePath(a.SyncFolder, prev.SyncFolder)
	res.Moved = res.Rerun && (!sameFolder || !SamePath(a.WoWDir, prev.WoWDir))
	var d *syncdir.Dir
	// syncdir.ErrAlreadyInitialized / ErrNotInitialized come back as is; callers add the hint
	// that fits them ("run `wtfsync link`" or "choose Join my other PC").
	if a.First && !sameFolder {
		d, err = syncdir.Init(a.SyncFolder, env.Now(), env.Hydrator)
	} else {
		d, err = syncdir.Open(a.SyncFolder, env.Hydrator)
	}
	if err != nil {
		return res, err
	}

	all := true
	s := local.Settings{MachineUUID: local.NewUUID(), MachineName: a.Name, SyncFolder: a.SyncFolder, WoWDir: a.WoWDir, Flavors: map[string]bool{}, AllFlavors: &all}
	if res.Rerun {
		s.MachineUUID = prev.MachineUUID
		maps.Copy(s.Flavors, prev.Flavors)
		s.AllFlavors, s.Appearance = prev.AllFlavors, prev.Appearance
	}
	// Listing every flavor found gives the Settings page's switches a starting point. When the
	// user picks flavors individually, one found later stays off (unlisted).
	if s.All() {
		for _, f := range flavors {
			if _, ok := s.Flavors[f.Name]; !ok {
				s.Flavors[f.Name] = true
			}
		}
	}
	if res.Moved {
		for f := range s.Flavors {
			if err := env.Store.SaveState(f, local.FlavorState{}); err != nil {
				return res, err
			}
		}
	}
	if err := d.WriteMachine(syncdir.MachineRecord{UUID: s.MachineUUID, Name: a.Name, LastSeen: env.Now()}); err != nil {
		return res, err
	}
	if err := env.Store.SaveSettings(s); err != nil {
		return res, err
	}
	res.Settings = s
	if res.Rerun && !res.Moved {
		return res, nil
	}

	eng := &engine.Engine{Store: env.Store, Settings: s, Procs: env.Procs, Hydrator: env.Hydrator, Prompt: env.Prompt, Now: env.Now}
	var errs []error
	for _, f := range flavors {
		if !s.Enabled(f.Name) {
			continue
		}
		run := eng.Pull
		if a.First {
			run = eng.Push
		}
		rep, err := run(f, engine.Options{})
		res.Synced = append(res.Synced, FlavorResult{Flavor: f.Name, Report: rep, Err: err})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
		}
	}
	return res, errors.Join(errs...)
}
