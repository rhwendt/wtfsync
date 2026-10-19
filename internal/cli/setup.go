package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// setup runs `init` (first PC: create the sync data, then push) or `link` (another PC: join
// existing sync data, then pull) by asking for setup.Answers in the terminal. On a PC that's
// already set up the current answers are the defaults (see package setup for the rerun rules).
func (c *cmd) setup(args []string, isInit bool) error {
	op := map[bool]string{true: "init", false: "link"}[isInit]
	fl, err := c.parse(op, args)
	if err != nil {
		return err
	}
	store := local.Store{Dir: fl.dataDir}
	prev, err := store.LoadSettings()
	if err != nil && !errors.Is(err, local.ErrNoSettings) {
		return err
	}

	p := c.prompter()
	wowDir, err := c.chooseWoWDir(p, fl.wowDir, prev.WoWDir)
	if err != nil {
		return err
	}
	flavors, err := setup.Flavors(wowDir)
	if err != nil {
		return err
	}
	if err := c.describe(wowDir, flavors, fl.dryRun); err != nil {
		return err
	}
	folder, err := c.chooseSyncFolder(p, fl.syncDir, prev.SyncFolder)
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
		def := prev.MachineName
		if def == "" {
			def, _ = c.deps.Hostname()
		}
		if name, err = p.line("Name for this PC", def); err != nil {
			return err
		}
	}

	l, err := lock.Acquire(fl.dataDir)
	if err != nil {
		return err
	}
	defer l.Release()
	env := setup.Env{Store: store, Procs: c.deps.Procs, Hydrator: c.deps.Hydrator, Prompt: c.prompter(), Now: c.deps.Now}
	res, err := setup.Run(env, setup.Answers{WoWDir: wowDir, SyncFolder: folder, Name: name, First: isInit})
	switch {
	case errors.Is(err, syncdir.ErrAlreadyInitialized):
		return fmt.Errorf("%w: run `wtfsync link` on this PC instead", err)
	case errors.Is(err, syncdir.ErrNotInitialized):
		return fmt.Errorf("%w: run `wtfsync init` on your first PC", err)
	case res.Settings.MachineUUID == "":
		return err // nothing was saved
	}
	fmt.Fprintf(c.out, "This PC is %q.\n", name)
	if res.Rerun && !res.Moved {
		fmt.Fprintln(c.out, "Settings updated; sync history kept.")
	}
	for _, r := range res.Synced {
		if r.Err == nil {
			printReport(c.out, map[bool]string{true: "push", false: "pull"}[isInit], r.Report, false)
		}
	}
	return err
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

func (c *cmd) chooseWoWDir(p prompter, flagVal, current string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if current != "" {
		return p.line("WoW install folder", current)
	}
	found := setup.FindInstalls(c.deps.InstallDirs)
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
	return pick(p, found, "1")
}

// pick asks for a number from a listed set of folders, or a typed folder.
func pick(p prompter, list []string, def string) (string, error) {
	a, err := p.line("Pick a number or type a folder", def)
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

func (c *cmd) chooseSyncFolder(p prompter, flagVal, current string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	cands := syncdir.Candidates(c.deps.Getenv, fsutil.IsDir)
	if len(cands) == 0 {
		return p.line(`Folder your cloud client syncs (e.g. G:\My Drive)`, current)
	}
	fmt.Fprintln(c.out, "Cloud folders found:")
	def := "1"
	if current != "" {
		def = current
	}
	for i, f := range cands {
		fmt.Fprintf(c.out, "  %d) %s\n", i+1, f)
		if current != "" && setup.SamePath(f, current) {
			def = strconv.Itoa(i + 1)
		}
	}
	return pick(p, cands, def)
}
