package cli

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// setup runs `init` (first PC: create the sync data, then push) or `link` (another PC: join
// existing sync data, then pull).
func (c *cmd) setup(args []string, isInit bool) error {
	op := map[bool]string{true: "init", false: "link"}[isInit]
	fl, err := c.parse(op, args)
	if err != nil {
		return err
	}
	store := local.Store{Dir: fl.dataDir}
	if _, err := store.LoadSettings(); err == nil {
		return fmt.Errorf("this PC is already set up (%s)", filepath.Join(fl.dataDir, "settings.json"))
	} else if !errors.Is(err, local.ErrNoSettings) {
		return err
	}

	p := c.prompter()
	wowDir, err := c.chooseWoWDir(p, fl.wowDir)
	if err != nil {
		return err
	}
	flavors, err := wow.Flavors(wowDir)
	if err != nil {
		return err
	}
	if len(flavors) == 0 {
		return fmt.Errorf("no WoW flavors (folders like _retail_ with a WTF folder) in %s", wowDir)
	}
	if err := c.describe(wowDir, flavors, fl.dryRun); err != nil {
		return err
	}
	folder, err := c.chooseSyncFolder(p, fl.syncDir)
	if err != nil {
		return err
	}
	if fl.dryRun {
		state := "no wtfsync data yet"
		if _, err := syncdir.Open(folder, c.deps.Hydrator); err == nil {
			state = "has wtfsync data"
		}
		fmt.Fprintf(c.out, "Sync folder: %s (%s)\nDry run: nothing written.\n", folder, state)
		return nil
	}

	name := fl.name
	if name == "" {
		host, _ := c.deps.Hostname()
		if name, err = p.line("Name for this PC", host); err != nil {
			return err
		}
	}

	var d *syncdir.Dir
	if isInit {
		d, err = syncdir.Init(folder, c.deps.Now(), c.deps.Hydrator)
		if errors.Is(err, syncdir.ErrAlreadyInitialized) {
			return fmt.Errorf("%w: run `wtfsync link` on this PC instead", err)
		}
	} else {
		d, err = syncdir.Open(folder, c.deps.Hydrator)
		if errors.Is(err, syncdir.ErrNotInitialized) {
			return fmt.Errorf("%w: run `wtfsync init` on your first PC", err)
		}
	}
	if err != nil {
		return err
	}

	s := local.Settings{MachineUUID: local.NewUUID(), MachineName: name, SyncFolder: folder, WoWDir: wowDir, Flavors: map[string]bool{}}
	for _, f := range flavors {
		s.Flavors[f.Name] = true
	}
	if err := d.WriteMachine(syncdir.MachineRecord{UUID: s.MachineUUID, Name: name, LastSeen: c.deps.Now()}); err != nil {
		return err
	}
	if err := store.SaveSettings(s); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "This PC is %q.\n", name)

	eng := c.engine(store, s)
	var errs []error
	for _, f := range flavors {
		var rep engine.Report
		if isInit {
			rep, err = eng.Push(f, engine.Options{})
		} else {
			rep, err = eng.Pull(f, engine.Options{})
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
			continue
		}
		printReport(c.out, map[bool]string{true: "push", false: "pull"}[isInit], rep, false)
	}
	return errors.Join(errs...)
}

// describe prints the install and how many files each flavor has (and lists them when asked).
func (c *cmd) describe(wowDir string, flavors []wow.Flavor, listFiles bool) error {
	fmt.Fprintf(c.out, "WoW install: %s\n", wowDir)
	for _, f := range flavors {
		files, err := collect.Collect(f.WTFDir())
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "  %s: %d SavedVariables file(s)\n", f.Name, len(files))
		if listFiles {
			for _, p := range slices.Sorted(maps.Keys(files)) {
				fmt.Fprintf(c.out, "    %s\n", p)
			}
		}
	}
	return nil
}

func (c *cmd) chooseWoWDir(p prompter, flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	var found []string
	for _, d := range c.deps.InstallDirs {
		if fl, err := wow.Flavors(d); err == nil && len(fl) > 0 {
			found = append(found, d)
		}
	}
	switch len(found) {
	case 0:
		return p.line("WoW install folder", "")
	case 1:
		return p.line("WoW install folder", found[0])
	}
	fmt.Fprintln(c.out, "WoW installs found:")
	for i, d := range found {
		fmt.Fprintf(c.out, "  %d) %s\n", i+1, d)
	}
	return pick(p, found)
}

// pick asks for a number from a listed set of folders, or a typed folder.
func pick(p prompter, list []string) (string, error) {
	a, err := p.line("Pick a number or type a folder", "1")
	if err != nil {
		return "", err
	}
	if i, err := strconv.Atoi(a); err == nil {
		if i < 1 || i > len(list) {
			return "", fmt.Errorf("no folder numbered %d", i)
		}
		return list[i-1], nil
	}
	return a, nil
}

func (c *cmd) chooseSyncFolder(p prompter, flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	cands := syncdir.Candidates(c.deps.Getenv, fsutil.IsDir)
	if len(cands) == 0 {
		return p.line(`Folder your cloud client syncs (e.g. G:\My Drive)`, "")
	}
	fmt.Fprintln(c.out, "Cloud folders found:")
	for i, f := range cands {
		fmt.Fprintf(c.out, "  %d) %s\n", i+1, f)
	}
	return pick(p, cands)
}
