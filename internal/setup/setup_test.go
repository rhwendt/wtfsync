package setup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

const sv = "Account/123#1/SavedVariables/ElvUI.lua"

type noProcs struct{}

func (noProcs) Running(wow.Flavor) (bool, error) { return false, nil }

type pc struct {
	env    Env
	wowDir string
}

func newPC(t *testing.T) *pc {
	t.Helper()
	root := t.TempDir()
	clock := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	p := &pc{wowDir: filepath.Join(root, "wow"), env: Env{
		Store:    local.Store{Dir: filepath.Join(root, "data")},
		Procs:    noProcs{},
		Hydrator: syncdir.TimeoutHydrator{Timeout: time.Second},
		Now:      func() time.Time { clock = clock.Add(time.Minute); return clock },
	}}
	p.write("a")
	return p
}

func (p *pc) write(content string) {
	path := filepath.Join(p.wowDir, "_retail_", "WTF", filepath.FromSlash(sv))
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0o644)
}

func (p *pc) answers(sync string, first bool) Answers {
	return Answers{WoWDir: p.wowDir, SyncFolder: sync, Name: "NJ", First: first}
}

func TestFirstRunPushesEveryFlavor(t *testing.T) {
	p, sync := newPC(t), t.TempDir()
	res, err := Run(p.env, p.answers(sync, true))
	if err != nil {
		t.Fatal(err)
	}
	if res.Rerun || len(res.Synced) != 1 || res.Synced[0].Flavor != "retail" || res.Synced[0].Report.Snapshot == "" {
		t.Fatalf("result %+v", res)
	}
	if s, _ := p.env.Store.LoadSettings(); s.MachineUUID == "" || !s.Flavors["retail"] {
		t.Fatalf("settings %+v", s)
	}
}

func TestRerunWithSameFoldersOnlySavesSettings(t *testing.T) {
	p, sync := newPC(t), t.TempDir()
	first, _ := Run(p.env, p.answers(sync, true))
	a := p.answers(sync, true)
	a.Name = "NJ desk"
	res, err := Run(p.env, a)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Rerun || res.Moved || res.Synced != nil {
		t.Fatalf("result %+v", res)
	}
	if res.Settings.MachineUUID != first.Settings.MachineUUID || res.Settings.MachineName != "NJ desk" {
		t.Fatalf("settings %+v", res.Settings)
	}
}

func TestRerunKeepsDisabledFlavorsOff(t *testing.T) {
	p, sync := newPC(t), t.TempDir()
	first, _ := Run(p.env, p.answers(sync, true))
	s := first.Settings
	off := false
	s.AllFlavors = &off
	s.Flavors["retail"] = false
	p.env.Store.SaveSettings(s)
	res, err := Run(p.env, p.answers(t.TempDir(), true)) // moved: would push, but retail is off
	if err != nil {
		t.Fatal(err)
	}
	if res.Settings.Flavors["retail"] || len(res.Synced) != 0 {
		t.Fatalf("result %+v", res)
	}
}

func TestMovingSyncFolderClearsHistory(t *testing.T) {
	p, sync := newPC(t), t.TempDir()
	Run(p.env, p.answers(sync, true))
	res, err := Run(p.env, p.answers(t.TempDir(), true))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot == "" {
		t.Fatalf("result %+v", res)
	}
	if st, _ := p.env.Store.LoadState("retail"); st.Base != res.Synced[0].Report.Snapshot {
		t.Fatalf("base %q, want the new folder's first snapshot", st.Base)
	}
}

func TestInitOnFolderWithDataAndLinkOnEmptyFolder(t *testing.T) {
	nj, ny, sync := newPC(t), newPC(t), t.TempDir()
	Run(nj.env, nj.answers(sync, true))
	if _, err := Run(ny.env, ny.answers(sync, true)); !errors.Is(err, syncdir.ErrAlreadyInitialized) {
		t.Fatalf("init on used folder: %v", err)
	}
	if _, err := Run(ny.env, ny.answers(t.TempDir(), false)); !errors.Is(err, syncdir.ErrNotInitialized) {
		t.Fatalf("link on empty folder: %v", err)
	}
	if _, err := ny.env.Store.LoadSettings(); !errors.Is(err, local.ErrNoSettings) {
		t.Fatalf("failed setup saved settings: %v", err)
	}
}

func TestLinkAbortedStopsAndReports(t *testing.T) {
	nj, ny, sync := newPC(t), newPC(t), t.TempDir()
	Run(nj.env, nj.answers(sync, true))
	ny.write("a-ny")
	ny.env.Prompt = abort{}
	res, err := Run(ny.env, ny.answers(sync, false))
	if !errors.Is(err, engine.ErrAborted) || len(res.Synced) != 1 || !errors.Is(res.Synced[0].Err, engine.ErrAborted) {
		t.Fatalf("res %+v, err %v", res, err)
	}
}

func TestFindInstallsSkipsNonWoWFolders(t *testing.T) {
	p := newPC(t)
	empty := t.TempDir()
	got := FindInstalls([]string{empty, filepath.Join(empty, "missing"), p.wowDir})
	if len(got) != 1 || got[0] != p.wowDir {
		t.Fatalf("got %v", got)
	}
	if _, err := Flavors(empty); err == nil {
		t.Fatal("want error for a folder without flavors")
	}
}

func TestSamePath(t *testing.T) {
	if !SamePath(`D:\World of Warcraft\`, `d:\world of warcraft`) && filepath.Separator == '\\' {
		t.Fatal("Windows paths differing in case and trailing slash should match")
	}
	if !SamePath("/a/b/", "/a/b") || SamePath("/a/b", "/a/c") {
		t.Fatal("SamePath")
	}
}

type abort struct{}

func (abort) FirstLink(int, string) (engine.FirstLinkChoice, error) { return 0, engine.ErrAborted }
func (abort) Conflict(string, string) (engine.Choice, error)        { return 0, engine.ErrAborted }

func TestFirstSetupSyncsAllFlavorsAndRerunKeepsAppearance(t *testing.T) {
	p, sync := newPC(t), t.TempDir()
	first, err := Run(p.env, p.answers(sync, true))
	if err != nil {
		t.Fatal(err)
	}
	if first.Settings.AllFlavors == nil || !*first.Settings.AllFlavors {
		t.Fatalf("first setup: allFlavors = %v, want true", first.Settings.AllFlavors)
	}
	s := first.Settings
	s.Appearance = "dark"
	p.env.Store.SaveSettings(s)
	res, err := Run(p.env, p.answers(sync, true))
	if err != nil {
		t.Fatal(err)
	}
	if res.Settings.Appearance != "dark" || !res.Settings.All() {
		t.Fatalf("rerun settings %+v", res.Settings)
	}
}
