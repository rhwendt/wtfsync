package engine

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// Status is a flavor's sync state as `wtfsync status` shows it.
type Status struct {
	Flavor       string
	Base         string // snapshot last synced; "" = never
	BaseBy       string // display name of the PC that wrote Base; "" when unknown
	Newest       string // newest snapshot in the sync folder; "" = none (or unreachable)
	NewestBy     string // display name of the PC that wrote Newest
	Reachable    bool
	LocalChanges int // files changed here since Base
	PendingPush  bool

	MissingAddons []OtherAddons // addons other PCs have that this one doesn't

	Points []RestorePoint // the snapshots kept in the sync folder, newest first; nil when unreachable
}

// RestorePoint is one snapshot this PC could go back to.
type RestorePoint struct {
	Name    string    // snapshot name
	Time    time.Time // when it was pushed, UTC
	From    string    // display name of the PC that pushed it; the start of its id when unknown
	Current bool      // this PC's files match it
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
	hashes := collect.Hashes(files)
	s := Status{Flavor: f.Name, Base: st.Base, PendingPush: st.PendingPush, LocalChanges: changed(st.Files, hashes)}
	d, err := syncdir.Open(e.Settings.SyncFolder, e.Hydrator)
	if errors.Is(err, syncdir.ErrUnreachable) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s.Reachable = true
	s.MissingAddons = e.missingAddons(f, d)
	if s.Points, err = restorePoints(d, f.Name, st, hashes, s.LocalChanges); err != nil {
		return s, err
	}
	if b, ok := syncdir.ParseName(s.Base); ok {
		if m, err := d.Machine(b.UUID); err == nil {
			s.BaseBy = m.Name
		}
	}
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

// restorePoints lists flavor's snapshots newest first. The current one comes from local state, so
// no snapshot is opened: the one restored while the files still match it, else the base when
// nothing changed since.
func restorePoints(d *syncdir.Dir, flavor string, st local.FlavorState, hashes map[string]string, changes int) ([]RestorePoint, error) {
	all, err := d.Snapshots(flavor)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	machines, _ := d.Machines() // unreadable records just show the PC's id
	for _, m := range machines {
		names[m.UUID] = m.Name
	}
	cur := ""
	switch {
	case st.Restored != "" && maps.Equal(st.RestoredFiles, hashes):
		cur = st.Restored
	case changes == 0:
		cur = st.Base
	}
	out := []RestorePoint{}
	for _, n := range slices.Backward(all) {
		from := names[n.UUID]
		if from == "" {
			from = n.UUID[:8]
		}
		out = append(out, RestorePoint{Name: n.String(), Time: n.Time, From: from, Current: n.String() == cur})
	}
	return out, nil
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
	if rep.Undo, err = e.Store.Backup(f.Name, f.WTFDir(), paths, e.Now(), local.Why{Reason: local.ReasonRestore}); err != nil {
		return rep, err
	}
	if err := e.Store.Restore(f.Name, id, f.WTFDir()); err != nil {
		return rep, err
	}
	return rep, e.Store.PruneBackups(f.Name) // only now: the set restored may have been the oldest
}
