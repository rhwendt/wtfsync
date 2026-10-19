package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

const (
	svA = "Account/123#1/SavedVariables/ElvUI.lua"
	svB = "Account/123#1/SavedVariables/Details.lua"
	svC = "Account/123#1/Area 52/Thrâll/SavedVariables/WeakAuras.lua"
)

type procs struct{ running bool }

func (p *procs) Running(wow.Flavor) (bool, error) { return p.running, nil }

// pc is one simulated PC: its own WoW install and data folder, sharing a sync folder with the others.
type pc struct {
	t       *testing.T
	name    string
	wowDir  string
	dataDir string
	sync    string
	procs   *procs
	clock   *time.Time
}

func newPC(t *testing.T, name, sync string, clock *time.Time) *pc {
	t.Helper()
	root := t.TempDir()
	p := &pc{t: t, name: name, wowDir: filepath.Join(root, "wow"), dataDir: filepath.Join(root, "data"), sync: sync, procs: &procs{}, clock: clock}
	if err := os.MkdirAll(filepath.Join(p.wowDir, "_retail_", "WTF", "Account"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// newPair returns NJ and NY sharing one sync folder and one clock; neither is set up yet.
func newPair(t *testing.T) (nj, ny *pc) {
	sync := t.TempDir()
	clock := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	return newPC(t, "NJ", sync, &clock), newPC(t, "NY", sync, &clock)
}

// linkedPair sets NJ up with files, links NY, and returns both in sync.
func linkedPair(t *testing.T, files map[string]string) (nj, ny *pc) {
	t.Helper()
	nj, ny = newPair(t)
	for p, c := range files {
		nj.write(p, c)
	}
	nj.initCmd()
	if code, out := ny.link(""); code != 0 {
		t.Fatalf("link: exit %d\n%s", code, out)
	}
	return nj, ny
}

func (p *pc) deps() Deps {
	return Deps{
		Procs:          p.procs,
		Hydrator:       syncdir.TimeoutHydrator{Timeout: 5 * time.Second},
		Now:            func() time.Time { *p.clock = p.clock.Add(time.Minute); return *p.clock },
		Hostname:       func() (string, error) { return "DESKTOP-" + p.name, nil },
		Getenv:         func(string) string { return "" },
		DefaultDataDir: p.dataDir,
		InstallDirs:    []string{p.wowDir},
	}
}

// run runs wtfsync on this PC with the given stdin; it returns the exit code and combined output.
func (p *pc) run(stdin string, args ...string) (int, string) {
	var out bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &out, p.deps())
	return code, out.String()
}

func (p *pc) mustRun(args ...string) string {
	p.t.Helper()
	code, out := p.run("", args...)
	if code != 0 {
		p.t.Fatalf("%s: wtfsync %v: exit %d\n%s", p.name, args, code, out)
	}
	return out
}

func (p *pc) initCmd() string {
	p.t.Helper()
	return p.mustRun("init", "--wow-dir", p.wowDir, "--sync-dir", p.sync, "--name", p.name)
}

func (p *pc) link(stdin string) (int, string) {
	return p.run(stdin, "link", "--wow-dir", p.wowDir, "--sync-dir", p.sync, "--name", p.name)
}

func (p *pc) path(rel string) string {
	return filepath.Join(p.wowDir, "_retail_", "WTF", filepath.FromSlash(rel))
}

func (p *pc) write(rel, content string) {
	p.t.Helper()
	os.MkdirAll(filepath.Dir(p.path(rel)), 0o755)
	if err := os.WriteFile(p.path(rel), []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p *pc) remove(rel string) {
	p.t.Helper()
	if err := os.Remove(p.path(rel)); err != nil {
		p.t.Fatal(err)
	}
}

func (p *pc) read(rel string) (string, bool) {
	p.t.Helper()
	b, err := os.ReadFile(p.path(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		p.t.Fatal(err)
	}
	return string(b), true
}

func (p *pc) want(rel, content string) {
	p.t.Helper()
	if got, ok := p.read(rel); !ok || got != content {
		p.t.Fatalf("%s: %s = %q (exists %v), want %q", p.name, rel, got, ok, content)
	}
}

func contains(t *testing.T, out, sub string) {
	t.Helper()
	if !strings.Contains(out, sub) {
		t.Fatalf("output missing %q:\n%s", sub, out)
	}
}
