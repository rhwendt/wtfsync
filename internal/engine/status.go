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

// Restore puts back a backup set (the newest when id is "") and returns the id restored.
func (e *Engine) Restore(f wow.Flavor, id string) (string, error) {
	if err := e.checkNotRunning(f); err != nil {
		return "", err
	}
	if id == "" {
		sets, err := e.Store.BackupSets(f.Name)
		if err != nil {
			return "", err
		}
		if len(sets) == 0 {
			return "", fmt.Errorf("no backups for %s yet", f.Name)
		}
		id = sets[0]
	}
	return id, e.Store.Restore(f.Name, id, f.WTFDir())
}
