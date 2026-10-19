package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

var (
	ErrGameRunning = errors.New("game is running")
	ErrAborted     = errors.New("aborted")
)

// Choice settles one conflicting file.
type Choice int

const (
	ChooseRemote Choice = iota // keep the other PC's version
	ChooseLocal                // keep this PC's version
)

// FirstLinkChoice settles all conflicts the first time a PC syncs a flavor (there's no base yet,
// so every file that differs would otherwise be its own conflict).
type FirstLinkChoice int

const (
	AllRemote FirstLinkChoice = iota
	AllLocal
	PerFile
)

// Prompter asks the user to settle conflicts. Returning ErrAborted stops without changes.
type Prompter interface {
	FirstLink(conflicts int, other string) (FirstLinkChoice, error)
	Conflict(path, other string) (Choice, error)
}

type Engine struct {
	Store    local.Store
	Settings local.Settings
	Procs    wow.Processes
	Hydrator syncdir.Hydrator
	Prompt   Prompter
	Now      func() time.Time
}

type Options struct {
	DryRun bool // report what would happen; change nothing
}

// Report describes what a pull or push did (or, with DryRun, would do).
type Report struct {
	Flavor   string
	UpToDate bool // pull: already had the newest snapshot; push: nothing changed since the base

	// Set when a snapshot was merged in (by pull, or by push when this PC was behind).
	Merged        string
	From          string // display name of the PC that wrote Merged
	Items         []Item // per-file decisions, conflicts already resolved (unresolved in a dry run)
	Backup        string // backup set id for files overwritten or deleted
	MissingAddons []string

	// Push only.
	Snapshot      string // snapshot written
	Pushed        int    // files changed since the base
	Pending       bool   // sync folder unreachable; retried later
	PendingReason string
}

func (e *Engine) checkNotRunning(f wow.Flavor) error {
	running, err := e.Procs.Running(f)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("%w: close WoW (%s) first", ErrGameRunning, f.Name)
	}
	return nil
}

// Pull brings the newest snapshot for f into the local WTF folder.
func (e *Engine) Pull(f wow.Flavor, opt Options) (Report, error) {
	rep := Report{Flavor: f.Name}
	if err := e.checkNotRunning(f); err != nil {
		return rep, err
	}
	d, err := syncdir.Open(e.Settings.SyncFolder, e.Hydrator)
	if err != nil {
		return rep, err
	}
	st, err := e.Store.LoadState(f.Name)
	if err != nil {
		return rep, err
	}
	newest, ok, err := d.Newest(f.Name)
	if err != nil {
		return rep, err
	}
	if !ok || newest.String() == st.Base {
		rep.UpToDate = true
		return rep, nil
	}
	_, err = e.merge(f, d, st, newest, opt, &rep)
	return rep, err
}

// merge applies snapshot n to the local WTF folder with the three-way rule and returns the new
// state (base = n). Nothing is written until every conflict is settled. With DryRun it only fills
// rep and returns st unchanged.
func (e *Engine) merge(f wow.Flavor, d *syncdir.Dir, st local.FlavorState, n syncdir.SnapshotName, opt Options, rep *Report) (local.FlavorState, error) {
	data, err := d.ReadSnapshot(f.Name, n)
	if err != nil {
		return st, err
	}
	m, remote, err := archive.Read(bytes.NewReader(data))
	if err != nil {
		return st, fmt.Errorf("snapshot %s: %w", n, err)
	}
	localFiles, err := collect.Collect(f.WTFDir())
	if err != nil {
		return st, err
	}
	installed, err := collect.Addons(f.AddOnsDir())
	if err != nil {
		return st, err
	}
	rep.Merged, rep.From = n.String(), m.Machine.Name
	rep.MissingAddons = collect.Missing(installed, m.Addons)
	base, known := mergeBase(d, f.Name, st, n, m)
	items := Plan(base, collect.Hashes(localFiles), m.Files)
	if !known {
		for i := range items {
			items[i].Action = Conflict
		}
	}
	if opt.DryRun {
		rep.Items = items
		return st, nil
	}
	if items, err = e.resolve(items, st.Base == "", m.Machine.Name); err != nil {
		return st, err
	}
	rep.Items = items

	var take []string
	for _, it := range items {
		if it.Action == TakeRemote {
			take = append(take, it.Path)
		}
	}
	if len(take) > 0 {
		if rep.Backup, err = e.Store.Backup(f.Name, f.WTFDir(), take, e.Now()); err != nil {
			return st, err
		}
		for _, p := range take {
			if err := apply(f.WTFDir(), p, remote); err != nil {
				return st, err
			}
		}
	}
	st.Base, st.Files = n.String(), m.Files
	return st, e.Store.SaveState(f.Name, st)
}

// resolve turns every Conflict into TakeRemote or KeepLocal by asking the Prompter. On a first
// link it asks once for all conflicts, unless the user picks "ask per file".
func (e *Engine) resolve(items []Item, firstLink bool, other string) ([]Item, error) {
	conflicts := 0
	for _, it := range items {
		if it.Action == Conflict {
			conflicts++
		}
	}
	if conflicts == 0 {
		return items, nil
	}
	all := Conflict // Conflict = ask per file
	if firstLink {
		c, err := e.Prompt.FirstLink(conflicts, other)
		if err != nil {
			return nil, err
		}
		switch c {
		case AllRemote:
			all = TakeRemote
		case AllLocal:
			all = KeepLocal
		}
	}
	out := make([]Item, len(items))
	copy(out, items)
	for i := range out {
		if out[i].Action != Conflict {
			continue
		}
		if all != Conflict {
			out[i].Action = all
			continue
		}
		c, err := e.Prompt.Conflict(out[i].Path, other)
		if err != nil {
			return nil, err
		}
		out[i].Action = KeepLocal
		if c == ChooseRemote {
			out[i].Action = TakeRemote
		}
	}
	return out, nil
}

// apply writes the snapshot's version of path, or deletes the local file if the snapshot has none.
func apply(wtfDir, path string, remote map[string][]byte) error {
	dst := filepath.Join(wtfDir, filepath.FromSlash(path))
	if b, ok := remote[path]; ok {
		return fsutil.WriteAtomic(dst, b)
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
