package engine

import (
	"errors"
	"fmt"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// Status is a flavor's sync state as `wtfsync status` shows it.
type Status struct {
	Flavor       string
	Base         string // snapshot last synced; "" = never
	Newest       string // newest snapshot in the sync folder; "" = none (or unreachable)
	NewestBy     string // display name of the PC that wrote Newest
	Reachable    bool
	LocalChanges int // files changed here since Base
	PendingPush  bool
}

func (e *Engine) Status(f wow.Flavor) (Status, error) {
	st, err := e.Store.LoadState(f.Name)
	if err != nil {
		return Status{}, err
	}
	files, err := collect.Collect(f.WTFDir())
	if err != nil {
		return Status{}, err
	}
	s := Status{Flavor: f.Name, Base: st.Base, PendingPush: st.PendingPush, LocalChanges: changed(st.Files, collect.Hashes(files))}
	d, err := syncdir.Open(e.Settings.SyncFolder, e.Hydrator)
	if errors.Is(err, syncdir.ErrUnreachable) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s.Reachable = true
	n, ok, err := d.Newest(f.Name)
	if err != nil || !ok {
		return s, err
	}
	s.Newest, s.NewestBy = n.String(), n.UUID
	if m, err := d.Machine(n.UUID); err == nil {
		s.NewestBy = m.Name
	}
	return s, nil
}

// RestoreReport describes what Restore did (or, with DryRun, would do).
type RestoreReport struct {
	ID    string   // backup set restored
	Paths []string // files it rewrote or deleted
	Undo  string   // backup set holding the files as they were just before; "" in a dry run
}

// Restore puts back a backup set (the newest when id is ""). The files it is about to overwrite or
// delete are backed up first, so a restore can itself be undone.
func (e *Engine) Restore(f wow.Flavor, id string, opt Options) (RestoreReport, error) {
	if err := e.checkNotRunning(f); err != nil {
		return RestoreReport{}, err
	}
	if id == "" {
		sets, err := e.Store.BackupSets(f.Name)
		if err != nil {
			return RestoreReport{}, err
		}
		if len(sets) == 0 {
			return RestoreReport{}, fmt.Errorf("no backups for %s yet", f.Name)
		}
		id = sets[0]
	}
	paths, err := e.Store.BackupPaths(f.Name, id)
	if err != nil {
		return RestoreReport{}, err
	}
	rep := RestoreReport{ID: id, Paths: paths}
	if opt.DryRun {
		return rep, nil
	}
	if rep.Undo, err = e.Store.Backup(f.Name, f.WTFDir(), paths, e.Now()); err != nil {
		return rep, err
	}
	return rep, e.Store.Restore(f.Name, id, f.WTFDir())
}
