package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// sync runs sync, push, pull, status, or restore for each selected flavor.
func (c *cmd) sync(op string, args []string) error {
	fl, err := c.parse(op, args)
	if err != nil {
		return err
	}
	store := local.Store{Dir: fl.dataDir}
	s, err := store.LoadSettings()
	if err != nil {
		return err
	}
	if fl.wowDir != "" {
		s.WoWDir = fl.wowDir
	}
	if fl.syncDir != "" {
		s.SyncFolder = fl.syncDir
	}
	flavors, err := selectFlavors(s, fl.flavor)
	if err != nil {
		return err
	}
	eng := c.engine(store, s)
	if !fl.dryRun {
		// One sync at a time per PC (CLI or tray). status only needs the lock for its retry,
		// so a busy tray doesn't stop it from showing where things stand.
		l, err := lock.Acquire(fl.dataDir)
		switch {
		case err == nil:
			defer l.Release()
			if op == "pull" || op == "status" {
				c.retryPending(eng, flavors)
			}
		case op != "status":
			return err
		}
	}
	opt := engine.Options{DryRun: fl.dryRun}
	var errs []error
	for _, f := range flavors {
		if err := c.one(eng, op, f, fl, opt); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
		}
	}
	return errors.Join(errs...)
}

func (c *cmd) one(eng *engine.Engine, op string, f wow.Flavor, fl flags, opt engine.Options) error {
	switch op {
	case "sync", "push", "pull":
		run, verb := eng.Push, "push" // sync is push: push merges what other PCs pushed first
		if op == "pull" {
			run, verb = eng.Pull, "pull"
		}
		r, err := run(f, opt)
		if err != nil {
			return err
		}
		printReport(c.out, verb, r, opt.DryRun)
	case "status":
		st, err := eng.Status(f)
		if err != nil {
			return err
		}
		printStatus(c.out, st)
	case "restore":
		r, err := eng.Restore(f, fl.set, opt)
		if err != nil {
			return err
		}
		verb := "restored"
		if opt.DryRun {
			verb = "would restore"
		}
		fmt.Fprintf(c.out, "%s: %s backup %s\n", f.Name, verb, r.ID)
		for _, p := range r.Paths {
			fmt.Fprintf(c.out, "  %s\n", p)
		}
		if r.Undo != "" {
			fmt.Fprintf(c.out, "  undo with: wtfsync restore --flavor %s --set %s\n", f.Name, r.Undo)
		}
		sets, err := eng.Store.BackupSets(f.Name)
		if err != nil {
			return err
		}
		if others := slices.DeleteFunc(sets, func(s string) bool { return s == r.ID || s == r.Undo }); len(others) > 0 {
			fmt.Fprintf(c.out, "  older backups (use --set): %s\n", strings.Join(others, ", "))
		}
	}
	return nil
}

// retryPending re-runs pushes that couldn't reach the sync folder last time.
func (c *cmd) retryPending(eng *engine.Engine, flavors []wow.Flavor) {
	for _, f := range flavors {
		st, err := eng.Store.LoadState(f.Name)
		if err != nil || !st.PendingPush {
			continue
		}
		fmt.Fprintf(c.out, "%s: retrying pending push\n", f.Name)
		r, err := eng.Push(f, engine.Options{})
		if err != nil {
			fmt.Fprintf(c.errw, "%s: %v\n", f.Name, err)
			continue
		}
		printReport(c.out, "push", r, false)
	}
}

// selectFlavors returns the installed flavors to act on: just name if given, else every enabled
// flavor (local.Settings.Enabled).
func selectFlavors(s local.Settings, name string) ([]wow.Flavor, error) {
	all, err := wow.Flavors(s.WoWDir)
	if err != nil {
		return nil, err
	}
	var out []wow.Flavor
	var names []string
	for _, f := range all {
		names = append(names, f.Name)
		if name != "" {
			if f.Name == name {
				return []wow.Flavor{f}, nil
			}
			continue
		}
		if s.Enabled(f.Name) {
			out = append(out, f)
		}
	}
	if name != "" {
		return nil, fmt.Errorf("unknown flavor %q (installed: %s)", name, strings.Join(names, ", "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no enabled flavors in %s", s.WoWDir)
	}
	return out, nil
}
