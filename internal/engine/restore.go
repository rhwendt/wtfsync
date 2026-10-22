package engine

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// ErrNoRestorePoint means the snapshot asked for isn't in the sync folder: pruned since, or never.
var ErrNoRestorePoint = errors.New("no such restore point")

// RestoreSnapshot makes this PC's synced files for f equal snapshot name: it writes the files the
// snapshot has and deletes synced files it doesn't. Files wtfsync doesn't sync are left alone.
// What it replaces is backed up first (Report.Undo). The base is unchanged, so the next sync
// uploads the restored files for the other PCs.
func (e *Engine) RestoreSnapshot(f wow.Flavor, name string, opt Options) (RestoreReport, error) {
	rep := RestoreReport{ID: name}
	if err := e.checkNotRunning(f); err != nil {
		return rep, err
	}
	d, err := syncdir.Open(e.Settings.SyncFolder, e.Hydrator)
	if err != nil {
		return rep, err
	}
	all, err := d.Snapshots(f.Name)
	if err != nil {
		return rep, err
	}
	i := slices.IndexFunc(all, func(n syncdir.SnapshotName) bool { return n.String() == name })
	if i < 0 {
		return rep, fmt.Errorf("%w: %q for %s", ErrNoRestorePoint, name, f.Name)
	}
	data, err := d.ReadSnapshot(f.Name, all[i])
	if err != nil {
		return rep, err
	}
	m, remote, err := archive.Read(bytes.NewReader(data))
	if err != nil {
		return rep, fmt.Errorf("snapshot %s: %w", name, err)
	}
	files, err := collect.Collect(f.WTFDir())
	if err != nil {
		return rep, err
	}
	hashes := collect.Hashes(files)
	if err := caseClash(hashes, m.Files); err != nil {
		return rep, err
	}
	for p, h := range m.Files {
		if hashes[p] != h {
			rep.Paths = append(rep.Paths, p)
		}
	}
	for p := range hashes {
		if _, ok := m.Files[p]; !ok {
			rep.Paths = append(rep.Paths, p)
		}
	}
	slices.Sort(rep.Paths)
	if opt.DryRun {
		return rep, nil
	}
	if len(rep.Paths) > 0 {
		if rep.Undo, err = e.Store.Backup(f.Name, f.WTFDir(), rep.Paths, e.Now(), local.Why{Reason: local.ReasonRestore}); err != nil {
			return rep, err
		}
		if err := e.Store.PruneBackups(f.Name); err != nil {
			return rep, err
		}
		for _, p := range rep.Paths {
			if err := apply(f.WTFDir(), p, remote); err != nil {
				return rep, fmt.Errorf("%w (what was there is saved: wtfsync restore --flavor %s --set %s)", err, f.Name, rep.Undo)
			}
		}
	}
	st, err := e.Store.LoadState(f.Name)
	if err != nil {
		return rep, err
	}
	st.Restored, st.RestoredFiles = name, m.Files
	return rep, e.Store.SaveState(f.Name, st)
}
