package engine

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/archive"
	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

const (
	pathA  = "Account/123#1/SavedVariables/ElvUI.lua"
	pathB  = "Account/123#1/SavedVariables/Details.lua"
	pathC  = "Account/123#1/Area 52/Thrâll/SavedVariables/WeakAuras.lua"
	njUUID = "00000000-0000-4000-8000-000000000001"
	nyUUID = "00000000-0000-4000-8000-000000000002"
)

type fakeProcs struct{ running bool }

func (p *fakeProcs) Running(wow.Flavor) (bool, error) { return p.running, nil }

// scripted answers prompts from queues and records which files it was asked about.
type scripted struct {
	firstLink []FirstLinkChoice
	choices   []Choice
	asked     []string
}

func (s *scripted) FirstLink(int, string) (FirstLinkChoice, error) {
	if len(s.firstLink) == 0 {
		return 0, ErrAborted
	}
	c := s.firstLink[0]
	s.firstLink = s.firstLink[1:]
	return c, nil
}

func (s *scripted) Conflict(path, _ string) (Choice, error) {
	s.asked = append(s.asked, path)
	if len(s.choices) == 0 {
		return 0, ErrAborted
	}
	c := s.choices[0]
	s.choices = s.choices[1:]
	return c, nil
}

// env is this PC ("NY") plus a shared sync folder that another PC ("NJ") writes snapshots into.
type env struct {
	t      *testing.T
	eng    *Engine
	procs  *fakeProcs
	prompt *scripted
	flavor wow.Flavor
	sync   *syncdir.Dir
	clock  time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	v := &env{
		t:      t,
		procs:  &fakeProcs{},
		prompt: &scripted{},
		flavor: wow.Flavor{Name: "retail", Dir: filepath.Join(root, "wow", "_retail_")},
		clock:  time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}
	if err := os.MkdirAll(v.flavor.WTFDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "drive")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	h := syncdir.TimeoutHydrator{Timeout: 5 * time.Second}
	d, err := syncdir.Init(folder, v.clock, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WriteMachine(syncdir.MachineRecord{UUID: njUUID, Name: "NJ", LastSeen: v.clock}); err != nil {
		t.Fatal(err)
	}
	v.sync = d
	v.eng = &Engine{
		Store: local.Store{Dir: filepath.Join(root, "data")},
		Settings: local.Settings{
			MachineUUID: nyUUID, MachineName: "NY", SyncFolder: folder,
			WoWDir: filepath.Join(root, "wow"), Flavors: map[string]bool{"retail": true},
		},
		Procs:    v.procs,
		Hydrator: h,
		Prompt:   v.prompt,
		Now:      func() time.Time { v.clock = v.clock.Add(time.Minute); return v.clock },
	}
	return v
}

func (v *env) writeLocal(path, content string) {
	v.t.Helper()
	p := filepath.Join(v.flavor.WTFDir(), filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		v.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		v.t.Fatal(err)
	}
}

func (v *env) removeLocal(path string) {
	v.t.Helper()
	if err := os.Remove(filepath.Join(v.flavor.WTFDir(), filepath.FromSlash(path))); err != nil {
		v.t.Fatal(err)
	}
}

func (v *env) local(path string) (string, bool) {
	v.t.Helper()
	b, err := os.ReadFile(filepath.Join(v.flavor.WTFDir(), filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		v.t.Fatal(err)
	}
	return string(b), true
}

// remote writes a snapshot from NJ with the given seq and file contents and returns its name.
func (v *env) remote(seq int, files map[string]string, addons ...collect.Addon) syncdir.SnapshotName {
	v.t.Helper()
	v.clock = v.clock.Add(time.Minute)
	n := syncdir.SnapshotName{Seq: seq, Time: v.clock, UUID: njUUID}
	content := map[string][]byte{}
	for p, s := range files {
		content[p] = []byte(s)
	}
	m := archive.Manifest{Machine: archive.Machine{UUID: njUUID, Name: "NJ"}, Created: v.clock, Flavor: "retail", Seq: seq, Addons: addons}
	var buf bytes.Buffer
	if err := archive.Write(&buf, m, content); err != nil {
		v.t.Fatal(err)
	}
	if err := v.sync.WriteSnapshot("retail", n, buf.Bytes()); err != nil {
		v.t.Fatal(err)
	}
	return n
}

// synced records that this PC last synced with snapshot n, whose files had these contents.
func (v *env) synced(n syncdir.SnapshotName, files map[string]string) {
	v.t.Helper()
	hashes := map[string]string{}
	for p, s := range files {
		hashes[p] = collect.Hash([]byte(s))
	}
	if err := v.eng.Store.SaveState("retail", local.FlavorState{Base: n.String(), Files: hashes}); err != nil {
		v.t.Fatal(err)
	}
}

func (v *env) state() local.FlavorState {
	v.t.Helper()
	st, err := v.eng.Store.LoadState("retail")
	if err != nil {
		v.t.Fatal(err)
	}
	return st
}

func (v *env) want(path, content string) {
	v.t.Helper()
	got, ok := v.local(path)
	if !ok || got != content {
		v.t.Fatalf("%s = %q (exists %v), want %q", path, got, ok, content)
	}
}
