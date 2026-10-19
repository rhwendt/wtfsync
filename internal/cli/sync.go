package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// sync runs push, pull, status, or restore for each selected flavor.
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
	if (op == "pull" || op == "status") && !fl.dryRun {
		c.retryPending(eng, flavors)
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
	case "push", "pull":
		run := eng.Push
		if op == "pull" {
			run = eng.Pull
		}
		r, err := run(f, opt)
		if err != nil {
			return err
		}
		printReport(c.out, op, r, opt.DryRun)
	case "status":
		st, err := eng.Status(f)
		if err != nil {
			return err
		}
		printStatus(c.out, st)
	case "restore":
		id, err := eng.Restore(f, fl.set)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "%s: restored backup %s\n", f.Name, id)
		sets, err := eng.Store.BackupSets(f.Name)
		if err != nil {
			return err
		}
		if others := slices.DeleteFunc(sets, func(s string) bool { return s == id }); len(others) > 0 {
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
// flavor. Flavors installed after setup aren't in settings yet and count as enabled.
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
		if enabled, known := s.Flavors[f.Name]; !known || enabled {
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
