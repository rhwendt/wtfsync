package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/update"
	"github.com/rhwendt/wtfsync/internal/wow"
)

const (
	svA = "Account/123#1/SavedVariables/ElvUI.lua"
	svB = "Account/123#1/SavedVariables/Details.lua"
)

type procs struct{ running bool }

func (p *procs) Running(wow.Flavor) (bool, error) { return p.running, nil }

// prompt answers the conflict dialog: every conflict gets choice, or err (e.g. Cancel).
type prompt struct {
	choice engine.Choice
	err    error
}

func (p prompt) FirstLink(int, string) (engine.FirstLinkChoice, error) {
	return engine.AllRemote, p.err
}
func (p prompt) Conflict(string, string) (engine.Choice, error) { return p.choice, p.err }

type notices struct {
	mu   sync.Mutex
	list []Notice
}

func (n *notices) add(x Notice) { n.mu.Lock(); n.list = append(n.list, x); n.mu.Unlock() }

func (n *notices) ids() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []string
	for _, x := range n.list {
		out = append(out, x.ID)
	}
	return out
}

func (n *notices) find(t *testing.T, id string) Notice {
	t.Helper()
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, x := range n.list {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("no notice %q in %v", id, n.list)
	return Notice{}
}

// pc is one simulated PC sharing a sync folder and clock with the others.
type pc struct {
	t       *testing.T
	name    string
	wowDir  string
	sync    string
	store   local.Store
	procs   *procs
	clock   *time.Time
	prompt  prompt
	notices *notices
	auto    *fakeAutostart
	updates *fakeUpdates
}

func newPair(t *testing.T) (nj, ny *pc) {
	sync := t.TempDir()
	clock := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	mk := func(name string) *pc {
		root := t.TempDir()
		p := &pc{t: t, name: name, wowDir: filepath.Join(root, "wow"), sync: sync, store: local.Store{Dir: filepath.Join(root, "data")},
			procs: &procs{}, clock: &clock, notices: &notices{}, auto: &fakeAutostart{}, updates: &fakeUpdates{}}
		os.MkdirAll(filepath.Join(p.wowDir, "_retail_", "WTF", "Account"), 0o755)
		return p
	}
	return mk("NJ"), mk("NY")
}

func (p *pc) now() time.Time { *p.clock = p.clock.Add(time.Minute); return *p.clock }

func (p *pc) app() *App {
	return New(Deps{
		Store: p.store, Procs: p.procs, Hydrator: syncdir.TimeoutHydrator{Timeout: time.Second},
		Prompt: p.prompt, Now: p.now, Updates: p.updates, Autostart: p.auto, Version: "v0.5.0",
		Notify: p.notices.add,
	})
}

func (p *pc) answers(first bool) setup.Answers {
	return setup.Answers{WoWDir: p.wowDir, SyncFolder: p.sync, Name: p.name, First: first}
}

// setUp runs setup through the app (init when first, else link).
func (p *pc) setUp(a *App, first bool) {
	p.t.Helper()
	if _, err := a.Setup(p.answers(first)); err != nil {
		p.t.Fatalf("%s setup: %v", p.name, err)
	}
}

func (p *pc) path(rel string) string {
	return filepath.Join(p.wowDir, "_retail_", "WTF", filepath.FromSlash(rel))
}

func (p *pc) write(rel, content string) {
	os.MkdirAll(filepath.Dir(p.path(rel)), 0o755)
	if err := os.WriteFile(p.path(rel), []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p *pc) want(rel, content string) {
	p.t.Helper()
	b, err := os.ReadFile(p.path(rel))
	if err != nil || string(b) != content {
		p.t.Fatalf("%s %s = %q (%v), want %q", p.name, rel, b, err, content)
	}
}

func (p *pc) install(folder, toc string) {
	dir := filepath.Join(p.wowDir, "_retail_", "Interface", "AddOns", folder)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, folder+".toc"), []byte(toc), 0o644)
}

// linked sets NJ up with files and links NY; both apps are returned in sync.
func linked(t *testing.T) (nj, ny *pc, njApp, nyApp *App) {
	nj, ny = newPair(t)
	nj.write(svA, "a0")
	nj.write(svB, "b0")
	njApp, nyApp = nj.app(), ny.app()
	nj.setUp(njApp, true)
	ny.setUp(nyApp, false)
	return nj, ny, njApp, nyApp
}

type fakeAutostart struct {
	on   bool
	sets int
}

func (f *fakeAutostart) Enabled() (bool, error) { return f.on, nil }
func (f *fakeAutostart) Set(on bool) error      { f.on, f.sets = on, f.sets+1; return nil }

type fakeUpdates struct {
	latest   update.Release
	err      error
	path     string
	dlErr    error
	download int
}

func (f *fakeUpdates) Latest(context.Context) (update.Release, error) { return f.latest, f.err }

func (f *fakeUpdates) Download(_ context.Context, r update.Release, dir string) (string, error) {
	f.download++
	if f.dlErr != nil {
		return "", f.dlErr
	}
	return filepath.Join(dir, "wtfsync-setup-"+r.Version+".exe"), nil
}

var errOffline = errors.New("offline")
