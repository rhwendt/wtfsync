package engine

import (
	"bytes"
	"errors"
	"time"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

const keepSnapshots = 20

// Push uploads this PC's SavedVariables for f as a new snapshot. If another PC pushed since this
// one last synced, that snapshot is merged first so its changes aren't hidden. If the sync folder
// can't be reached the push is marked pending (Report.Pending) rather than failing: the data is the
// local files, which stay put, so the retry just pushes again.
func (e *Engine) Push(f wow.Flavor, opt Options) (Report, error) {
	rep := Report{Flavor: f.Name}
	if err := e.checkNotRunning(f); err != nil {
		return rep, err
	}
	st, err := e.Store.LoadState(f.Name)
	if err != nil {
		return rep, err
	}
	d, err := syncdir.Open(e.Settings.SyncFolder, e.Hydrator)
	if errors.Is(err, syncdir.ErrUnreachable) {
		return e.pending(f, st, opt, rep, err)
	}
	if err != nil {
		return rep, err
	}
	newest, ok, err := d.Newest(f.Name)
	if err != nil {
		return rep, err
	}
	if ok && newest.String() != st.Base {
		if st, err = e.merge(f, d, st, newest, opt, &rep); err != nil {
			return rep, err
		}
	}

	files, err := collect.Collect(f.WTFDir())
	if err != nil {
		return rep, err
	}
	hashes := collect.Hashes(files)
	addons, err := collect.Addons(f.AddOnsDir())
	if err != nil {
		return rep, err
	}
	rep.Pushed = changed(st.Files, hashes)
	if rep.Pushed == 0 || opt.DryRun {
		rep.UpToDate = rep.Pushed == 0
		rep.MissingAddons = e.missingAddons(f, d)
		if opt.DryRun {
			return rep, nil
		}
		if err := e.recordAddons(f, d, addons); err != nil {
			return rep, err
		}
		if st.PendingPush {
			st.PendingPush = false
			return rep, e.Store.SaveState(f.Name, st)
		}
		return rep, nil
	}

	name := syncdir.SnapshotName{Seq: 1, Time: e.Now().UTC().Truncate(time.Second), UUID: e.Settings.MachineUUID}
	if ok {
		name.Seq = newest.Seq + 1
	}
	m := archive.Manifest{
		Machine: archive.Machine{UUID: e.Settings.MachineUUID, Name: e.Settings.MachineName},
		Created: name.Time,
		Flavor:  f.Name,
		Seq:     name.Seq,
		Base:    st.Base,
		Addons:  addons,
	}
	var buf bytes.Buffer
	if err := archive.Write(&buf, m, files); err != nil {
		return rep, err
	}
	if err := d.WriteSnapshot(f.Name, name, buf.Bytes()); err != nil {
		return e.pending(f, st, opt, rep, err)
	}
	if err := e.Store.SaveState(f.Name, local.FlavorState{Base: name.String(), Files: hashes}); err != nil {
		return rep, err
	}
	rep.Snapshot = name.String()
	record := syncdir.MachineRecord{UUID: e.Settings.MachineUUID, Name: e.Settings.MachineName, LastSeen: name.Time}
	if err := d.WriteMachine(record); err != nil {
		return rep, err
	}
	if err := e.recordAddons(f, d, addons); err != nil {
		return rep, err
	}
	rep.MissingAddons = e.missingAddons(f, d)
	return rep, d.Prune(f.Name, keepSnapshots, nil)
}

func (e *Engine) pending(f wow.Flavor, st local.FlavorState, opt Options, rep Report, cause error) (Report, error) {
	rep.Pending, rep.PendingReason = true, cause.Error()
	if opt.DryRun {
		return rep, nil
	}
	st.PendingPush = true
	return rep, e.Store.SaveState(f.Name, st)
}
