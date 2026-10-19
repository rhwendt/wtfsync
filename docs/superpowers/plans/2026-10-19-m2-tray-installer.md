# wtfsync Milestone 2 (Tray app, installer, updates) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A per-user installer (`wtfsync-setup.exe`) that installs the existing CLI plus a new tray app (`wtfsync-tray.exe`): tray icon and menu showing per-flavor sync state, windows for setup, conflicts, settings, status, missing addons and restore, toast notifications, and one-click updates from GitHub Releases.

**Architecture:** All decisions live in Linux-testable packages: `setup` (init/link shared by CLI and wizard), `lock` (one sync per PC), `update` (GitHub releases + checksum), `autostart`, `icons`, `applog`, and `app` (tray state, menu model, actions, tick, notification decisions). `ui` is thin Wails v3 glue (Windows-only files) plus plain HTML/JS pages; its conflict `prompter` is untagged and tested on Linux. CI builds both executables and the NSIS installer on Linux and attaches them to releases.

**Tech Stack:** Go 1.26, Wails `v3.0.0-beta.32` (tray, windows, notifications service, single instance), `golang.org/x/sys` (Windows file lock, registry), NSIS 3 (`makensis` on ubuntu), GitHub Actions.

**Spec:** [`docs/superpowers/specs/2026-10-19-m2-tray-installer-design.md`](../specs/2026-10-19-m2-tray-installer-design.md) (builds on [`2026-10-18-wtfsync-design.md`](../specs/2026-10-18-wtfsync-design.md))

## Global Constraints

- Two executables sharing all sync code: `wtfsync.exe` (console CLI, unchanged apart from `version`) and `wtfsync-tray.exe` (GUI subsystem, `-ldflags "-H windowsgui"`).
- Wails pinned to **`v3.0.0-beta.32`**; upgrades are deliberate PRs. Third-party dependencies: `golang.org/x/sys` and Wails (with what it pulls in). Nothing else.
- Front end: plain HTML/CSS/JS embedded with `go:embed`; no npm or bundler. Pages import from `/wails/runtime.js` and call Go as `Call.ByName("github.com/rhwendt/wtfsync/internal/ui.Bridge.<Method>", …)`.
- Everything Wails-specific is in `//go:build windows` files so `go vet ./...` and `go test ./...` keep working on Linux. Build Windows binaries with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64`.
- Wails must be created with `Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true}`, or closing the last window quits the tray.
- Installer: NSIS, `RequestExecutionLevel user`, installs to `%LOCALAPPDATA%\Programs\wtfsync\`; uninstall keeps `%APPDATA%\wtfsync`.
- Startup entry: HKCU `Software\Microsoft\Windows\CurrentVersion\Run`, value `wtfsync`, data `"<install dir>\wtfsync-tray.exe"` — written by both the installer and the tray's setting.
- Updates: `GET https://api.github.com/repos/rhwendt/wtfsync/releases/latest` at startup and every 24 h; assets `wtfsync-setup.exe` and `checksums.txt` (`sha256sum` format); never for version `dev`.
- One sync at a time per PC: `lock.Acquire(<data dir>)` around push, pull, restore, setup (CLI and tray). Held → CLI exit 2 "another wtfsync sync is running"; tray notification "A sync is already running."
- Version comes from `-ldflags "-X main.version=<v>"` (`v1.2.3` on release builds, `dev` otherwise); `wtfsync version` prints it.
- Tray tick every 60 s; tray log `%APPDATA%\wtfsync\tray.log`, kept under 1 MB with one previous file.
- Branch protection requires the CI checks `test (ubuntu-latest)`, `test (windows-latest)` and `build`: keep those job names.
- Work on branch `feature/m2-tray`. Conventional Commits. No `Claude-Session:` trailers or session URLs in commits/PRs; `Co-Authored-By` is fine.

**Plan decisions that refine the spec** (flagged for the reviewer):
1. The installer's "options page" is NSIS's stock components page with two optional sections (start with Windows, add to PATH), both on by default. On an upgrade, `.onInit` starts from the current choices, so a silent update never turns a disabled option back on.
2. When Settings moves the PC to a different cloud folder, the app joins it (link) if it already has wtfsync data and starts it (init) otherwise — no extra question.
3. `setup.Run` now only syncs **enabled** flavors on the first push/pull (the CLI used to push/pull all of them).
4. The tray's left-click opens the menu (Wails' default with no attached window); Status is a menu item.
5. `restore` in the tray restores one flavor at a time (a button per flavor in the Restore window).
6. Icons are drawn in code (four coloured circles), so there are no binary assets to commit.

## Review Focus

1. **Closing the conflict window with X or Alt+F4, or quitting the tray while it's open:** treated as Cancel; no file changes and the sync goroutine returns. (The prompter's cancel path is unit-tested in Task 9; the `WindowClosing` hook in Task 10 is what wires X to it — check it.)
2. **A silent update (`/S`) on a PC where the user turned off "start with Windows" or PATH:** stays off afterwards (Task 11 `.onInit`).
3. **User profile paths with spaces or non-ASCII letters** (`C:\Users\Jöhn Doe\…`) in the install dir, the Run value and PATH: every NSIS path is quoted; PATH edits only touch the exact `;<dir>;` entry (Task 11).
4. **Notifications that can't be shown** (Focus Assist, toast registration failing): the action still completes and the icon/tooltip still update; the error only goes to `tray.log` (Task 10 `notify`).
5. **Clicking Pull twice quickly, or the tick firing during an action:** the second action returns `ErrBusy` without a notification; the tick skips; nothing is left "busy" (Task 8 tests `TestActionWhileBusyIsRefused`, `TestTicksAndActionsAtOnce`).

## File Structure

```
.github/scripts/build.sh              build dist/: both exes, installer, checksums.txt
.github/workflows/ci.yml              version → build (installer) → release with assets
installer/wtfsync.nsi                 per-user NSIS installer
cmd/wtfsync/main.go                   + version
cmd/wtfsync-tray/main_windows.go      real deps → ui.Run
cmd/wtfsync-tray/main_other.go        "runs on Windows" stub
internal/lock/                        per-PC sync lock (flock / LockFileEx)
internal/engine/engine.go             + BatchPrompter
internal/setup/setup.go               init/link logic shared by CLI and wizard
internal/cli/setup.go                 prompts → setup.Run
internal/cli/sync.go, cli.go          + lock, + version
internal/update/update.go             latest release, Newer, Download + verify
internal/autostart/                   HKCU Run value (Windows), stub elsewhere
internal/icons/icons.go               tray icons drawn in code
internal/applog/applog.go             size-capped log file
internal/app/view.go                  IconState, View, tooltip, Menu
internal/app/present.go               rows/lines for the windows
internal/app/app.go                   App: Refresh, Tick, Push, Pull, Restore, Setup, Settings, updates
internal/ui/prompter.go               conflict dialog as engine.BatchPrompter (untagged, tested)
internal/ui/ui_windows.go             Wails app, tray, menu, windows, notifications
internal/ui/bridge_windows.go         Go methods the pages call
internal/ui/web/*.html, app.js, app.css
```

---

### Task 1: Per-PC sync lock

**Files:**
- Create: `internal/lock/lock.go`, `internal/lock/lock_unix.go`, `internal/lock/lock_windows.go`
- Test: `internal/lock/lock_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `var lock.ErrHeld`; `func lock.Acquire(dir string) (*lock.Lock, error)` (non-blocking; creates `dir`); `func (*Lock) Release() error` (safe to call twice).

- [ ] **Step 1: Create the branch and write the failing test** — `internal/lock/lock_test.go`

```bash
git switch main && git pull && git switch -c feature/m2-tray
```

```go
package lock

import (
	"errors"
	"testing"
)

func TestSecondAcquireFailsUntilRelease(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir); !errors.Is(err, ErrHeld) {
		t.Fatalf("second acquire: err = %v, want ErrHeld", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := Acquire(dir)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again.Release()
}

func TestAcquireCreatesDataFolder(t *testing.T) {
	l, err := Acquire(t.TempDir() + "/not/yet")
	if err != nil {
		t.Fatal(err)
	}
	l.Release()
}

func TestReleaseTwiceIsHarmless(t *testing.T) {
	l, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("second release: %v", err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/lock/`
Expected: FAIL — `undefined: Acquire`, `undefined: ErrHeld`.

- [ ] **Step 3: Implement**

`internal/lock/lock.go`:
```go
// Package lock makes sure only one wtfsync process syncs at a time on this PC (CLI or tray).
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrHeld = errors.New("another wtfsync sync is running")

// Lock is held until Release, or until the process exits (the OS drops file locks then too).
type Lock struct {
	f *os.File
}

// Acquire takes the lock in dir (wtfsync's data folder) without waiting. It returns ErrHeld if
// another process, or another Acquire in this one, holds it.
func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "sync.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlockFile(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	l.f = nil
	return err
}
```

`internal/lock/lock_unix.go`:
```go
//go:build !windows

package lock

import (
	"errors"
	"os"
	"syscall"
)

func lockFile(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrHeld
	}
	return err
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
```

`internal/lock/lock_windows.go`:
```go
//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(f *os.File) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrHeld
	}
	return err
}

func unlockFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
```

- [ ] **Step 4: Run the tests (Linux) and vet for Windows**

Run: `go test ./internal/lock/ && GOOS=windows go vet ./internal/lock/`
Expected: `ok`, no vet output. (The Windows lock runs in CI's `windows-latest` test job.)

- [ ] **Step 5: Commit**

```bash
git add internal/lock/
git commit -m "feat: add a per-PC lock so only one sync runs at a time"
```

---

### Task 2: Batch conflict prompts in the engine

**Files:**
- Modify: `internal/engine/engine.go` (after the `Prompter` interface; in `resolve`; new `resolveBatch` before `apply`)
- Test: `internal/engine/batch_test.go`

**Interfaces:**
- Consumes: `engine.Prompter`, `Choice`, `ErrAborted`, the engine test helpers (`newEnv`, `remote`, `synced`, `writeLocal`, `want`, `state`, `scripted`).
- Produces: `type engine.BatchPrompter interface { Resolve(conflicts []string, other string, firstLink bool) (map[string]Choice, error) }`. When `Engine.Prompt` implements it, all conflicts go to one `Resolve` call; a missing choice → error wrapping `ErrAborted`, nothing written.

- [ ] **Step 1: Write the failing test** — `internal/engine/batch_test.go`

```go
package engine

import (
	"errors"
	"reflect"
	"testing"
)

// batch answers every conflict at once, like the tray's conflict dialog.
type batch struct {
	scripted // a BatchPrompter is still a Prompter
	choices  map[string]Choice
	err      error
	asked    []string
	first    bool
}

func (b *batch) Resolve(conflicts []string, _ string, firstLink bool) (map[string]Choice, error) {
	b.asked, b.first = conflicts, firstLink
	return b.choices, b.err
}

// conflicted is NY synced at s1, then both NJ (s2) and this PC changed A and B.
func conflicted(t *testing.T, p Prompter) *env {
	v := newEnv(t)
	v.eng.Prompt = p
	s1 := v.remote(1, map[string]string{pathA: "a1", pathB: "b1"})
	v.synced(s1, map[string]string{pathA: "a1", pathB: "b1"})
	v.writeLocal(pathA, "a-mine")
	v.writeLocal(pathB, "b-mine")
	v.remote(2, map[string]string{pathA: "a-nj", pathB: "b-nj"})
	return v
}

func TestBatchPrompterSettlesAllConflictsInOneCall(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseRemote, pathB: ChooseLocal}}
	v := conflicted(t, b)
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if want := []string{pathB, pathA}; !reflect.DeepEqual(b.asked, want) || b.first {
		t.Fatalf("asked %v (firstLink %v), want %v", b.asked, b.first, want)
	}
	v.want(pathA, "a-nj")
	v.want(pathB, "b-mine")
}

func TestBatchPrompterMissingChoiceWritesNothing(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseRemote}} // nothing for B
	v := conflicted(t, b)
	before := v.state().Base
	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	v.want(pathA, "a-mine")
	if v.state().Base != before {
		t.Fatal("base moved")
	}
}

func TestBatchPrompterCancelWritesNothing(t *testing.T) {
	b := &batch{err: ErrAborted}
	v := conflicted(t, b)
	if _, err := v.eng.Pull(v.flavor, Options{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}
	v.want(pathA, "a-mine")
	v.want(pathB, "b-mine")
}

func TestBatchPrompterToldAboutFirstLink(t *testing.T) {
	b := &batch{choices: map[string]Choice{pathA: ChooseLocal}}
	v := newEnv(t)
	v.eng.Prompt = b
	v.writeLocal(pathA, "a-mine")
	v.remote(1, map[string]string{pathA: "a-nj"})
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if !b.first {
		t.Fatal("firstLink = false on a PC that never synced")
	}
	v.want(pathA, "a-mine")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/engine/ -run Batch`
Expected: FAIL — `aborted` in the tests that expect success: without batch support the engine falls back to the fake's per-file `Conflict`/`FirstLink`, whose empty queues abort.

- [ ] **Step 3: Implement** — in `internal/engine/engine.go`

After the `Prompter` interface, add:

```go
// BatchPrompter is an optional Prompter that settles every conflict in one go (the tray's
// conflict dialog). firstLink is true when this PC has never synced the flavor. Every path in
// conflicts must get a choice; a missing one, or an error, writes nothing.
type BatchPrompter interface {
	Resolve(conflicts []string, other string, firstLink bool) (map[string]Choice, error)
}
```

In `resolve`, right after `if conflicts == 0 { return items, nil }`, add:

```go
	if bp, ok := e.Prompt.(BatchPrompter); ok {
		return resolveBatch(bp, items, firstLink, other)
	}
```

Before `// apply writes the snapshot's version of path…`, add:

```go
func resolveBatch(bp BatchPrompter, items []Item, firstLink bool, other string) ([]Item, error) {
	var paths []string
	for _, it := range items {
		if it.Action == Conflict {
			paths = append(paths, it.Path)
		}
	}
	choices, err := bp.Resolve(paths, other, firstLink)
	if err != nil {
		return nil, err
	}
	out := make([]Item, len(items))
	copy(out, items)
	for i := range out {
		if out[i].Action != Conflict {
			continue
		}
		c, ok := choices[out[i].Path]
		if !ok {
			return nil, fmt.Errorf("%w: no choice for %s", ErrAborted, out[i].Path)
		}
		out[i].Action = KeepLocal
		if c == ChooseRemote {
			out[i].Action = TakeRemote
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/engine/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit -m "feat: let a prompter settle all conflicts in one call"
```

---

### Task 3: Shared setup package; CLI init/link on top of it

**Files:**
- Create: `internal/setup/setup.go`
- Replace: `internal/cli/setup.go`
- Test: `internal/setup/setup_test.go` (existing `internal/cli/setup_test.go` must keep passing unchanged)

**Interfaces:**
- Consumes: `local.Store`, `syncdir.Init`/`Open`/`ErrAlreadyInitialized`/`ErrNotInitialized`, `engine.Engine`, `wow.Flavors`, `lock.Acquire` (Task 1).
- Produces:
  - `type setup.Answers struct { WoWDir, SyncFolder, Name string; First bool }`
  - `type setup.Env struct { Store local.Store; Procs wow.Processes; Hydrator syncdir.Hydrator; Prompt engine.Prompter; Now func() time.Time }`
  - `type setup.FlavorResult struct { Flavor string; Report engine.Report; Err error }`
  - `type setup.Result struct { Settings local.Settings; Rerun, Moved bool; Synced []FlavorResult }`
  - `func setup.Run(env Env, a Answers) (Result, error)` — saves settings + machine record, first push (`First`) or pull; rerun keeps the UUID; moving folders clears state. `syncdir.ErrAlreadyInitialized` / `ErrNotInitialized` are returned unwrapped for callers to add hints. Nothing is saved on those errors.
  - `func setup.Flavors(wowDir string) ([]wow.Flavor, error)`, `func setup.FindInstalls(dirs []string) []string`, `func setup.SamePath(a, b string) bool`

- [ ] **Step 1: Write the failing test** — `internal/setup/setup_test.go`

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/setup/`
Expected: FAIL — `undefined: Env`, `undefined: Run`.

- [ ] **Step 3: Implement** — `internal/setup/setup.go`

```go
// Package setup sets a PC up for syncing (init and link), for both the CLI and the tray's wizard.
// Running it again on a PC that's set up keeps the PC's identity; moving it to another WoW folder
// or sync folder clears its sync history so the next sync asks like a first link.
package setup

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// Answers are what setup asks for.
type Answers struct {
	WoWDir     string
	SyncFolder string
	Name       string
	First      bool // init: create the sync data and push; otherwise link: join it and pull
}

// Env is what Run needs from outside.
type Env struct {
	Store    local.Store
	Procs    wow.Processes
	Hydrator syncdir.Hydrator
	Prompt   engine.Prompter
	Now      func() time.Time
}

// FlavorResult is the first push or pull of one flavor.
type FlavorResult struct {
	Flavor string
	Report engine.Report
	Err    error
}

type Result struct {
	Settings local.Settings
	Rerun    bool           // the PC was already set up
	Moved    bool           // a rerun that changed the WoW folder or sync folder
	Synced   []FlavorResult // first push (init) or pull (link); empty when only settings changed
}

// Flavors lists a WoW folder's flavors, or says why it isn't a WoW folder.
func Flavors(wowDir string) ([]wow.Flavor, error) {
	flavors, err := wow.Flavors(wowDir)
	if err != nil {
		return nil, err
	}
	if len(flavors) == 0 {
		return nil, fmt.Errorf("no WoW flavors (folders like _retail_) in %s", wowDir)
	}
	return flavors, nil
}

// FindInstalls returns the folders in dirs that hold at least one WoW flavor, in order.
func FindInstalls(dirs []string) []string {
	var found []string
	for _, d := range dirs {
		if fl, err := wow.Flavors(d); err == nil && len(fl) > 0 {
			found = append(found, d)
		}
	}
	return found
}

// SamePath compares folders the way Windows does: cleaned and ignoring letter case.
func SamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Run saves the answers and does the first push or pull. Errors from individual flavors are in
// Result.Synced and also returned joined, so callers can map them to exit codes.
func Run(env Env, a Answers) (Result, error) {
	var res Result
	prev, err := env.Store.LoadSettings()
	res.Rerun = err == nil
	if err != nil && !errors.Is(err, local.ErrNoSettings) {
		return res, err
	}
	flavors, err := Flavors(a.WoWDir)
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(a.Name) == "" {
		return res, errors.New("this PC needs a name")
	}

	sameFolder := res.Rerun && SamePath(a.SyncFolder, prev.SyncFolder)
	res.Moved = res.Rerun && (!sameFolder || !SamePath(a.WoWDir, prev.WoWDir))
	var d *syncdir.Dir
	// syncdir.ErrAlreadyInitialized / ErrNotInitialized come back as is; callers add the hint
	// that fits them ("run `wtfsync link`" or "choose Join my other PC").
	if a.First && !sameFolder {
		d, err = syncdir.Init(a.SyncFolder, env.Now(), env.Hydrator)
	} else {
		d, err = syncdir.Open(a.SyncFolder, env.Hydrator)
	}
	if err != nil {
		return res, err
	}

	s := local.Settings{MachineUUID: local.NewUUID(), MachineName: a.Name, SyncFolder: a.SyncFolder, WoWDir: a.WoWDir, Flavors: map[string]bool{}}
	if res.Rerun {
		s.MachineUUID = prev.MachineUUID
		maps.Copy(s.Flavors, prev.Flavors)
	}
	for _, f := range flavors {
		if _, ok := s.Flavors[f.Name]; !ok {
			s.Flavors[f.Name] = true
		}
	}
	if res.Moved {
		for f := range s.Flavors {
			if err := env.Store.SaveState(f, local.FlavorState{}); err != nil {
				return res, err
			}
		}
	}
	if err := d.WriteMachine(syncdir.MachineRecord{UUID: s.MachineUUID, Name: a.Name, LastSeen: env.Now()}); err != nil {
		return res, err
	}
	if err := env.Store.SaveSettings(s); err != nil {
		return res, err
	}
	res.Settings = s
	if res.Rerun && !res.Moved {
		return res, nil
	}

	eng := &engine.Engine{Store: env.Store, Settings: s, Procs: env.Procs, Hydrator: env.Hydrator, Prompt: env.Prompt, Now: env.Now}
	var errs []error
	for _, f := range flavors {
		if !s.Flavors[f.Name] {
			continue
		}
		run := eng.Pull
		if a.First {
			run = eng.Push
		}
		rep, err := run(f, engine.Options{})
		res.Synced = append(res.Synced, FlavorResult{Flavor: f.Name, Report: rep, Err: err})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
		}
	}
	return res, errors.Join(errs...)
}
```

- [ ] **Step 4: Replace `internal/cli/setup.go`** so `init`/`link` gather answers and call `setup.Run` under the lock

```go
package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// setup runs `init` (first PC: create the sync data, then push) or `link` (another PC: join
// existing sync data, then pull) by asking for setup.Answers in the terminal. On a PC that's
// already set up the current answers are the defaults (see package setup for the rerun rules).
func (c *cmd) setup(args []string, isInit bool) error {
	op := map[bool]string{true: "init", false: "link"}[isInit]
	fl, err := c.parse(op, args)
	if err != nil {
		return err
	}
	store := local.Store{Dir: fl.dataDir}
	prev, err := store.LoadSettings()
	if err != nil && !errors.Is(err, local.ErrNoSettings) {
		return err
	}

	p := c.prompter()
	wowDir, err := c.chooseWoWDir(p, fl.wowDir, prev.WoWDir)
	if err != nil {
		return err
	}
	flavors, err := setup.Flavors(wowDir)
	if err != nil {
		return err
	}
	if err := c.describe(wowDir, flavors, fl.dryRun); err != nil {
		return err
	}
	folder, err := c.chooseSyncFolder(p, fl.syncDir, prev.SyncFolder)
	if err != nil {
		return err
	}
	if fl.dryRun {
		state := "no wtfsync data yet"
		if _, err := syncdir.Open(folder, c.deps.Hydrator); err == nil {
			state = "has wtfsync data"
		}
		fmt.Fprintf(c.out, "Sync folder: %s (%s)\nDry run: nothing written.\n", folder, state)
		return nil
	}
	name := fl.name
	if name == "" {
		def := prev.MachineName
		if def == "" {
			def, _ = c.deps.Hostname()
		}
		if name, err = p.line("Name for this PC", def); err != nil {
			return err
		}
	}

	l, err := lock.Acquire(fl.dataDir)
	if err != nil {
		return err
	}
	defer l.Release()
	env := setup.Env{Store: store, Procs: c.deps.Procs, Hydrator: c.deps.Hydrator, Prompt: c.prompter(), Now: c.deps.Now}
	res, err := setup.Run(env, setup.Answers{WoWDir: wowDir, SyncFolder: folder, Name: name, First: isInit})
	switch {
	case errors.Is(err, syncdir.ErrAlreadyInitialized):
		return fmt.Errorf("%w: run `wtfsync link` on this PC instead", err)
	case errors.Is(err, syncdir.ErrNotInitialized):
		return fmt.Errorf("%w: run `wtfsync init` on your first PC", err)
	case res.Settings.MachineUUID == "":
		return err // nothing was saved
	}
	fmt.Fprintf(c.out, "This PC is %q.\n", name)
	if res.Rerun && !res.Moved {
		fmt.Fprintln(c.out, "Settings updated; sync history kept.")
	}
	for _, r := range res.Synced {
		if r.Err == nil {
			printReport(c.out, map[bool]string{true: "push", false: "pull"}[isInit], r.Report, false)
		}
	}
	return err
}

// describe prints the install and how many files each flavor has (and lists them when asked).
func (c *cmd) describe(wowDir string, flavors []wow.Flavor, listFiles bool) error {
	fmt.Fprintf(c.out, "WoW install: %s\n", wowDir)
	for _, f := range flavors {
		files, err := collect.Collect(f.WTFDir())
		if err != nil {
			return err
		}
		fmt.Fprintf(c.out, "  %s: %d SavedVariables file(s)\n", f.Name, len(files))
		if listFiles {
			for _, p := range slices.Sorted(maps.Keys(files)) {
				fmt.Fprintf(c.out, "    %s\n", p)
			}
		}
	}
	return nil
}

func (c *cmd) chooseWoWDir(p prompter, flagVal, current string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if current != "" {
		return p.line("WoW install folder", current)
	}
	found := setup.FindInstalls(c.deps.InstallDirs)
	switch len(found) {
	case 0:
		return p.line("WoW install folder", "")
	case 1:
		return p.line("WoW install folder", found[0])
	}
	fmt.Fprintln(c.out, "WoW installs found:")
	for i, d := range found {
		fmt.Fprintf(c.out, "  %d) %s\n", i+1, d)
	}
	return pick(p, found, "1")
}

// pick asks for a number from a listed set of folders, or a typed folder.
func pick(p prompter, list []string, def string) (string, error) {
	a, err := p.line("Pick a number or type a folder", def)
	if err != nil {
		return "", err
	}
	if i, err := strconv.Atoi(a); err == nil {
		if i < 1 || i > len(list) {
			return "", fmt.Errorf("no folder numbered %d", i)
		}
		return list[i-1], nil
	}
	return a, nil
}

func (c *cmd) chooseSyncFolder(p prompter, flagVal, current string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	cands := syncdir.Candidates(c.deps.Getenv, fsutil.IsDir)
	if len(cands) == 0 {
		return p.line(`Folder your cloud client syncs (e.g. G:\My Drive)`, current)
	}
	fmt.Fprintln(c.out, "Cloud folders found:")
	def := "1"
	if current != "" {
		def = current
	}
	for i, f := range cands {
		fmt.Fprintf(c.out, "  %d) %s\n", i+1, f)
		if current != "" && setup.SamePath(f, current) {
			def = strconv.Itoa(i + 1)
		}
	}
	return pick(p, cands, def)
}
```

- [ ] **Step 5: Run all tests**

Run: `go vet ./... && go test ./internal/setup/ ./internal/cli/`
Expected: both `ok` — the CLI's existing setup tests pass unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/setup/ internal/cli/setup.go
git commit -m "refactor: move init/link logic into a setup package shared by CLI and tray"
```

---

### Task 4: CLI takes the lock; `wtfsync version`

**Files:**
- Modify: `internal/cli/sync.go`, `internal/cli/cli.go`, `cmd/wtfsync/main.go`
- Test: `internal/cli/lock_test.go`

**Interfaces:**
- Consumes: `lock.Acquire`, `lock.ErrHeld` (Task 1); CLI test helpers (`linkedPair`, `run`, `mustRun`, `want`, `contains`, `start`, `svA`).
- Produces: `cli.Deps.Version string`; `version` / `--version` command; `lock.ErrHeld` → exit 2. `push`, `pull`, `restore` refuse while locked; `status` still answers (skips its retry).

- [ ] **Step 1: Write the failing test** — `internal/cli/lock_test.go`

```go
package cli

import (
	"testing"

	"github.com/rhwendt/wtfsync/internal/lock"
)

func TestSyncRefusedWhileAnotherSyncRuns(t *testing.T) {
	nj, ny := linkedPair(t, start)
	nj.write(svA, "a-nj")
	nj.mustRun("push")
	held, err := lock.Acquire(ny.dataDir) // e.g. the tray is pulling
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"pull", "push", "restore"} {
		code, out := ny.run("", op)
		if code != 2 {
			t.Fatalf("%s: exit %d, want 2\n%s", op, code, out)
		}
		contains(t, out, "another wtfsync sync is running")
	}
	ny.want(svA, "a0")
	contains(t, ny.mustRun("status"), "retail:") // status still answers
	held.Release()
	ny.mustRun("pull")
	ny.want(svA, "a-nj")
}

func TestVersion(t *testing.T) {
	nj, _ := newPair(t)
	contains(t, nj.mustRun("version"), "wtfsync dev")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/cli/ -run 'AnotherSync|Version'`
Expected: FAIL — `pull: exit 0, want 2` and `unknown command "version"`.

- [ ] **Step 3: Implement**

`internal/cli/sync.go` — add `"github.com/rhwendt/wtfsync/internal/lock"` to the imports and replace

```go
	if (op == "pull" || op == "status") && !fl.dryRun {
		c.retryPending(eng, flavors)
	}
```

with

```go
	if !fl.dryRun {
		// One sync at a time per PC (CLI or tray). status only needs the lock for its retry,
		// so a busy tray doesn't stop it from showing where things stand.
		l, err := lock.Acquire(fl.dataDir)
		switch {
		case err == nil:
			defer l.Release()
			if op == "pull" || op == "status" {
				c.retryPending(eng, flavors)
			}
		case op != "status":
			return err
		}
	}
```

`internal/cli/cli.go`:
- imports: add `"cmp"` and `"github.com/rhwendt/wtfsync/internal/lock"`;
- `Deps`: add `Version        string   // set at build time; "dev" otherwise` after `InstallDirs`;
- `exitRefused` comment: `// game running, snapshot not downloaded yet, or another sync running`;
- usage, after the `restore` line: `  version   print wtfsync's version`;
- in `Run`'s switch, before `case "help", …`:

```go
	case "version", "--version":
		fmt.Fprintln(stdout, "wtfsync", cmp.Or(deps.Version, "dev"))
		return exitOK
```

- in `exitCode`: `case errors.Is(err, engine.ErrGameRunning), errors.Is(err, syncdir.ErrNotHydrated), errors.Is(err, lock.ErrHeld):`

`cmd/wtfsync/main.go` — above `func main()`:

```go
// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"
```

and in the `cli.Deps{…}` literal add `Version:        version,` after `InstallDirs`.

- [ ] **Step 4: Run the tests and check the build flag**

Run: `go test ./internal/cli/ && go build -ldflags "-X main.version=v9.8.7" -o /tmp/wtfsync ./cmd/wtfsync && /tmp/wtfsync version`
Expected: `ok`, then `wtfsync v9.8.7`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ cmd/wtfsync/
git commit -m "feat: refuse CLI syncs while another runs; add wtfsync version"
```

---

### Task 5: Update checker

**Files:**
- Create: `internal/update/update.go`
- Test: `internal/update/update_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `const update.SetupAsset = "wtfsync-setup.exe"`, `ChecksumsAsset = "checksums.txt"`; `var update.ErrChecksum`; `type update.Release struct { Version, SetupURL, ChecksumsURL string }`; `type update.Checker struct { Client *http.Client; API string }`; `func update.GitHub() Checker`; `(Checker).Latest(ctx) (Release, error)`; `func update.Newer(current, latest string) bool`; `(Checker).Download(ctx, r Release, dir string) (string, error)` — returns `<dir>/wtfsync-setup-<version>.exe`, nothing left behind on failure.

- [ ] **Step 1: Write the failing test** — `internal/update/update_test.go`

```go
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeGitHub serves a latest release whose installer is setup and whose checksums file says sum.
func fakeGitHub(t *testing.T, tag, setup, sum string, assets ...string) Checker {
	t.Helper()
	if assets == nil {
		assets = []string{SetupAsset, ChecksumsAsset}
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repo/releases/latest":
			var list []string
			for _, a := range assets {
				list = append(list, fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/dl/%s"}`, a, srv.URL, a))
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, tag, strings.Join(list, ","))
		case "/dl/" + SetupAsset:
			fmt.Fprint(w, setup)
		case "/dl/" + ChecksumsAsset:
			fmt.Fprintf(w, "%s  wtfsync.exe\n%s  %s\n", strings.Repeat("0", 64), sum, SetupAsset)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return Checker{Client: srv.Client(), API: srv.URL + "/repo"}
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestLatestFindsInstallerAndChecksums(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ", sha("MZ"))
	r, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "v0.5.0" || !strings.HasSuffix(r.SetupURL, "/dl/"+SetupAsset) || !strings.HasSuffix(r.ChecksumsURL, "/dl/"+ChecksumsAsset) {
		t.Fatalf("release %+v", r)
	}
}

func TestLatestWithoutInstallerIsAnError(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ", sha("MZ"), "wtfsync.exe", ChecksumsAsset)
	if _, err := c.Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "no installer") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.4.0", "v0.5.0", true},
		{"v0.4.0", "v0.4.1", true},
		{"v0.9.9", "v1.0.0", true},
		{"v0.10.0", "v0.9.0", false}, // numeric, not string, order
		{"v0.5.0", "v0.5.0", false},
		{"v0.5.0", "v0.4.9", false},
		{"dev", "v9.9.9", false}, // development builds never update
		{"v0.5.0", "nightly", false},
	} {
		if got := Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ-installer", sha("MZ-installer"))
	r, _ := c.Latest(context.Background())
	dir := t.TempDir()
	path, err := c.Download(context.Background(), r, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "MZ-installer" {
		t.Fatalf("installer = %q", b)
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "tampered", sha("MZ-installer"))
	r, _ := c.Latest(context.Background())
	dir := t.TempDir()
	if _, err := c.Download(context.Background(), r, dir); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/update/`
Expected: FAIL — `undefined: Checker`, `undefined: Newer`.

- [ ] **Step 3: Implement** — `internal/update/update.go`

```go
// Package update finds newer wtfsync releases on GitHub and downloads their installer.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	SetupAsset     = "wtfsync-setup.exe"
	ChecksumsAsset = "checksums.txt"
)

var ErrChecksum = errors.New("downloaded installer doesn't match its checksum")

// Release is a published version that has an installer.
type Release struct {
	Version      string // "v0.5.0"
	SetupURL     string
	ChecksumsURL string
}

// Checker talks to the GitHub releases API.
type Checker struct {
	Client *http.Client
	API    string // https://api.github.com/repos/<owner>/<repo>
}

// GitHub checks this project's releases.
func GitHub() Checker {
	return Checker{Client: &http.Client{Timeout: 5 * time.Minute}, API: "https://api.github.com/repos/rhwendt/wtfsync"}
}

// Latest returns the newest published release. A release without both the installer and the
// checksums file is an error: there's nothing safe to install.
func (c Checker) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.API+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("check for updates: %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("check for updates: %w", err)
	}
	r := Release{Version: body.TagName}
	for _, a := range body.Assets {
		switch a.Name {
		case SetupAsset:
			r.SetupURL = a.URL
		case ChecksumsAsset:
			r.ChecksumsURL = a.URL
		}
	}
	if r.SetupURL == "" || r.ChecksumsURL == "" {
		return Release{}, fmt.Errorf("release %s has no installer", r.Version)
	}
	return r, nil
}

// Newer reports whether latest is a higher vMAJOR.MINOR.PATCH than current. Development builds
// (current "dev" or anything unparsable) never update.
func Newer(current, latest string) bool {
	c, ok1 := parse(current)
	l, ok2 := parse(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Download fetches r's installer into dir and checks it against r's checksums file. It returns
// the installer's path; on any failure nothing is left in dir.
func (c Checker) Download(ctx context.Context, r Release, dir string) (string, error) {
	sums, err := c.get(ctx, r.ChecksumsURL)
	if err != nil {
		return "", err
	}
	want, err := checksumFor(sums, SetupAsset)
	sums.Close()
	if err != nil {
		return "", err
	}
	body, err := c.get(ctx, r.SetupURL)
	if err != nil {
		return "", err
	}
	defer body.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "wtfsync-setup-"+r.Version+".exe")
	f, err := os.Create(dst + ".tmp")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = ErrChecksum
	}
	if err == nil {
		err = os.Rename(dst+".tmp", dst)
	}
	if err != nil {
		os.Remove(dst + ".tmp")
		return "", err
	}
	return dst, nil
}

func (c Checker) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// checksumFor reads `sha256sum` output ("<hex>  <name>" per line) and returns name's hash.
func checksumFor(r io.Reader, name string) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no checksum for %s", ChecksumsAsset, name)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/update/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/update/
git commit -m "feat: find newer releases and download a verified installer"
```

---

### Task 6: Tray support packages: autostart, icons, log file

**Files:**
- Create: `internal/autostart/autostart.go`, `autostart_windows.go`, `autostart_other.go`; `internal/icons/icons.go`; `internal/applog/applog.go`
- Test: `internal/autostart/autostart_windows_test.go`, `autostart_other_test.go`; `internal/icons/icons_test.go`; `internal/applog/applog_test.go`

**Interfaces:**
- Consumes: `golang.org/x/sys/windows/registry`.
- Produces:
  - `type autostart.Setting interface { Enabled() (bool, error); Set(on bool) error }`; `type autostart.Run struct { Name, Command string }` implements it (HKCU Run value on Windows; off / error elsewhere).
  - `icons.Idle`, `icons.Syncing`, `icons.NewSettings`, `icons.Attention` — `[]byte` 32×32 PNGs.
  - `const applog.DefaultMax = 1 << 20`; `func applog.Open(path string, max int64) (*applog.File, error)`; `(*File).Write`, `(*File).Close`.

- [ ] **Step 1: Write the failing tests**

`internal/autostart/autostart_windows_test.go` (runs in CI on `windows-latest`):
```go
//go:build windows

package autostart

import "testing"

func TestSetAndClearRunValue(t *testing.T) {
	r := Run{Name: "wtfsync-test-autostart", Command: `"C:\nowhere\wtfsync-tray.exe"`}
	t.Cleanup(func() { r.Set(false) })
	for _, on := range []bool{true, false, false} { // clearing twice is fine
		if err := r.Set(on); err != nil {
			t.Fatal(err)
		}
		if got, err := r.Enabled(); err != nil || got != on {
			t.Fatalf("after Set(%v): Enabled = %v, %v", on, got, err)
		}
	}
}
```

`internal/autostart/autostart_other_test.go`:
```go
//go:build !windows

package autostart

import "testing"

func TestUnsupportedOffWindows(t *testing.T) {
	r := Run{Name: "wtfsync"}
	if on, err := r.Enabled(); on || err != nil {
		t.Fatalf("Enabled = %v, %v", on, err)
	}
	if err := r.Set(true); err == nil {
		t.Fatal("Set(true) should fail off Windows")
	}
}
```

`internal/icons/icons_test.go`:
```go
package icons

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIconsAreDistinct32pxPNGs(t *testing.T) {
	seen := map[string]bool{}
	for name, b := range map[string][]byte{"idle": Idle, "syncing": Syncing, "new": NewSettings, "attention": Attention} {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if s := img.Bounds().Size(); s.X != 32 || s.Y != 32 {
			t.Fatalf("%s: size %v", name, s)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("%s: corner should be transparent", name)
		}
		c := img.At(16, 16)
		r, g, b, _ := c.RGBA()
		key := string([]byte{byte(r), byte(g), byte(b)})
		if seen[key] {
			t.Fatalf("%s: same colour as another icon", name)
		}
		seen[key] = true
	}
}
```

`internal/applog/applog_test.go`:
```go
package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatesPastCapKeepingOnePreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "tray.log")
	l, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"aaaaaa\n", "bbbbbb\n", "cccccc\n"} {
		if _, err := l.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()
	cur, _ := os.ReadFile(path)
	prev, _ := os.ReadFile(path + ".1")
	if string(cur) != "cccccc\n" || string(prev) != "bbbbbb\n" {
		t.Fatalf("current %q, previous %q", cur, prev)
	}
}

func TestReopenAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tray.log")
	for _, s := range []string{"one\n", "two\n"} {
		l, err := Open(path, DefaultMax)
		if err != nil {
			t.Fatal(err)
		}
		l.Write([]byte(s))
		l.Close()
	}
	if b, _ := os.ReadFile(path); !strings.HasPrefix(string(b), "one\ntwo\n") {
		t.Fatalf("log %q", b)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/autostart/ ./internal/icons/ ./internal/applog/`
Expected: FAIL — `undefined: Run`, `undefined: Idle`, `undefined: Open`.

- [ ] **Step 3: Implement**

`internal/autostart/autostart.go`:
```go
// Package autostart turns "start wtfsync when Windows starts" on and off. It uses the same
// HKCU Run value the installer's checkbox writes.
package autostart

// Run is one value under HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
type Run struct {
	Name    string // "wtfsync"
	Command string // `"C:\...\wtfsync-tray.exe"`
}

// Setting is what the tray's settings window needs; tests fake it.
type Setting interface {
	Enabled() (bool, error)
	Set(on bool) error
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
```

`internal/autostart/autostart_windows.go`:
```go
//go:build windows

package autostart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func (r Run) Enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue(r.Name)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (r Run) Set(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if on {
		return k.SetStringValue(r.Name, r.Command)
	}
	if err := k.DeleteValue(r.Name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
```

`internal/autostart/autostart_other.go`:
```go
//go:build !windows

package autostart

import "errors"

func (Run) Enabled() (bool, error) { return false, nil }

func (Run) Set(bool) error { return errors.New("start at login is only supported on Windows") }
```

`internal/icons/icons.go`:
```go
// Package icons draws the tray icons: a filled circle whose colour is the sync state.
package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	Idle        = circle(color.RGBA{0x2e, 0x9e, 0x5b, 0xff}) // green: in sync
	Syncing     = circle(color.RGBA{0x2f, 0x7f, 0xd8, 0xff}) // blue: working
	NewSettings = circle(color.RGBA{0xe8, 0xa3, 0x17, 0xff}) // amber: another PC has newer settings
	Attention   = circle(color.RGBA{0xd6, 0x3b, 0x3b, 0xff}) // red: needs you
)

const size = 32

func circle(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	const r = size/2 - 1
	for y := range size {
		for x := range size {
			dx, dy := x-size/2, y-size/2
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding an in-memory RGBA image can't fail
	}
	return buf.Bytes()
}
```

`internal/applog/applog.go`:
```go
// Package applog is the tray's log file: it keeps the current file under a size cap and one
// previous file (<name>.1).
package applog

import (
	"os"
	"path/filepath"
	"sync"
)

const DefaultMax = 1 << 20 // 1 MB

type File struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	size int64
}

// Open appends to path, creating its folder if needed.
func Open(path string, max int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	l := &File{path: path, max: max}
	return l, l.open()
}

func (l *File) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.f, l.size = f, fi.Size()
	return nil
}

// Write appends p, first moving the file to <path>.1 if p would take it past the cap.
func (l *File) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size > 0 && l.size+int64(len(p)) > l.max {
		l.f.Close()
		if err := os.Rename(l.path, l.path+".1"); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		if err := l.open(); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
```

- [ ] **Step 4: Run the tests; build the Windows test binary**

Run: `go test ./internal/autostart/ ./internal/icons/ ./internal/applog/ && GOOS=windows go vet ./internal/autostart/ && GOOS=windows go test -c -o /dev/null ./internal/autostart/`
Expected: three `ok`, no vet output, the Windows test binary builds.

- [ ] **Step 5: Commit**

```bash
git add internal/autostart/ internal/icons/ internal/applog/
git commit -m "feat: add start-at-login, tray icons and a size-capped log file"
```

---

### Task 7: Tray view model: icon, tooltip, menu, window text

**Files:**
- Create: `internal/app/view.go`, `internal/app/present.go`
- Test: `internal/app/view_test.go`, `internal/app/present_test.go`

**Interfaces:**
- Consumes: `engine.Status`, `engine.OtherAddons`, `engine.ErrGameRunning`/`ErrAborted`, `setup.Result` (Task 3), `collect.MissingAddon`.
- Produces:
  - `type app.IconState int` — `Idle`, `NewSettings`, `Attention`, `Syncing` (in increasing severity).
  - `type app.FlavorView struct { Name string; Status engine.Status; Err string }`
  - `type app.View struct { SetUp bool; Busy string; Icon IconState; Tooltip string; Flavors []FlavorView; Missing int; Update string }`
  - `func summarize(v *View)` (fills Icon, Tooltip, Missing; tooltip ≤ 127 runes); `const tooltipMax = 127`
  - `type app.MenuItem struct { ID, Label string; Enabled bool }`; `func app.Menu(v View) []MenuItem` — IDs `setup`, `push`, `pull`, `status`, `restore`, `addons`, `settings`, `update`, `quit`, `-` (separator).
  - `func friendly(flavor string, err error) string`
  - `app.StatusRow`, `func app.StatusRows(View) []StatusRow`; `app.AddonRow`, `func app.AddonRows(View) []AddonRow`; `func app.ResultLines(setup.Result) []string`

- [ ] **Step 1: Write the failing tests**

`internal/app/view_test.go`:
```go
package app

import (
	"strings"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
)

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}

func TestMenuDisablesSyncWhileBusy(t *testing.T) {
	v := View{SetUp: true, Busy: "pulling"}
	summarize(&v)
	if v.Icon != Syncing || v.Tooltip != "wtfsync — pulling…" {
		t.Fatalf("view %+v", v)
	}
	for _, it := range Menu(v) {
		want := it.ID == "status" || it.ID == "quit"
		if it.ID != "-" && it.Enabled != want {
			t.Errorf("%s enabled = %v while busy", it.ID, it.Enabled)
		}
	}
}

func TestTooltipFitsWindowsLimit(t *testing.T) {
	v := View{SetUp: true, Flavors: []FlavorView{
		{Name: "retail", Status: engine.Status{Reachable: true}, Err: string(make([]rune, 300))},
		{Name: "classic", Status: engine.Status{PendingPush: true, Reachable: true}},
	}}
	summarize(&v)
	if n := len([]rune(v.Tooltip)); n > tooltipMax {
		t.Fatalf("tooltip has %d runes", n)
	}
	contains(t, v.Tooltip, "…")
}
```

`internal/app/present_test.go`:
```go
package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/collect"
	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/setup"
)

func TestStatusRows(t *testing.T) {
	v := View{Flavors: []FlavorView{
		{Name: "classic", Status: engine.Status{Reachable: true, Base: "s1", Newest: "s1", NewestBy: "NY"}},
		{Name: "retail", Status: engine.Status{Reachable: true, Newest: "s2", NewestBy: "NJ", LocalChanges: 2}},
	}}
	want := []StatusRow{
		{Flavor: "classic", State: "in sync", Base: "s1", Newest: "s1 from NY", Local: "0 local change(s)"},
		{Flavor: "retail", State: "NJ has newer settings", Base: "never synced", Newest: "s2 from NJ", Local: "2 local change(s)"},
	}
	if got := StatusRows(v); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestAddonRows(t *testing.T) {
	v := View{Flavors: []FlavorView{{Name: "retail", Status: engine.Status{MissingAddons: []engine.OtherAddons{
		{Machine: "NJ", Addons: []collect.MissingAddon{{Name: "ElvUI", Folders: []string{"ElvUI", "ElvUI_Options"}, URL: "https://x"}}},
	}}}}}
	want := []AddonRow{{Flavor: "retail", Machine: "NJ", Name: "ElvUI", Folders: "ElvUI, ElvUI_Options", URL: "https://x"}}
	if got := AddonRows(v); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestResultLines(t *testing.T) {
	r := setup.Result{Synced: []setup.FlavorResult{
		{Flavor: "classic", Report: engine.Report{Snapshot: "s1", Pushed: 122}},
		{Flavor: "retail", Report: engine.Report{Merged: "s9", From: "NJ", Items: make([]engine.Item, 3)}},
		{Flavor: "ptr", Report: engine.Report{UpToDate: true}},
		{Flavor: "era", Err: errors.Join(engine.ErrGameRunning)},
	}}
	want := []string{"classic: uploaded 122 file(s)", "retail: got NJ's settings (3 file(s))", "ptr: nothing to do", "era: Close WoW (era) first"}
	if got := ResultLines(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if got := ResultLines(setup.Result{Rerun: true}); !reflect.DeepEqual(got, []string{"Settings saved; sync history kept."}) {
		t.Fatalf("rerun: %q", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/app/`
Expected: FAIL — `undefined: View`, `undefined: StatusRows`.

- [ ] **Step 3: Implement**

`internal/app/view.go`:
```go
// Package app is the tray app's logic, kept free of any UI toolkit so it can be tested anywhere:
// what state each flavor is in, which icon and menu to show, what the menu items do, the 60 s
// tick, update offers, and which notifications to raise. Package ui only renders it.
package app

import (
	"errors"
	"fmt"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// IconState is the tray icon, most severe state across flavors.
type IconState int

const (
	Idle        IconState = iota // every flavor in sync
	NewSettings                  // another PC pushed newer settings
	Attention                    // error, refused action, pending push, missing addons, not set up
	Syncing                      // an action is running
)

// FlavorView is one flavor as the tray shows it.
type FlavorView struct {
	Name   string
	Status engine.Status
	Err    string // the last action's error for this flavor; "" when it went fine
}

// newer reports whether another PC has pushed something this PC hasn't synced.
func (f FlavorView) newer() bool {
	return f.Status.Reachable && f.Status.Newest != "" && f.Status.Newest != f.Status.Base
}

func (f FlavorView) missing() int {
	n := 0
	for _, o := range f.Status.MissingAddons {
		n += len(o.Addons)
	}
	return n
}

// issue is what's wrong with the flavor in a few words, most important first; "" if nothing.
func (f FlavorView) issue() (string, IconState) {
	s := f.Status
	switch {
	case f.Err != "":
		return f.Err, Attention
	case s.PendingPush:
		return "push pending (sync folder unreachable)", Attention
	case !s.Reachable:
		return "sync folder unreachable", Attention
	case f.missing() > 0:
		return fmt.Sprintf("%d addon(s) missing here", f.missing()), Attention
	case f.newer():
		return s.NewestBy + " has newer settings", NewSettings
	}
	return "", Idle
}

// View is everything the tray renders.
type View struct {
	SetUp   bool
	Busy    string // "pulling", "pushing", "restoring", "setting up", "updating"; "" when idle
	Icon    IconState
	Tooltip string
	Flavors []FlavorView
	Missing int    // missing addons across flavors
	Update  string // newer version on offer, e.g. "v0.6.0"; "" when none
}

const tooltipMax = 127 // Windows truncates tray tooltips beyond this

func summarize(v *View) {
	switch {
	case !v.SetUp:
		v.Icon, v.Tooltip = Attention, "wtfsync — not set up"
		return
	case v.Busy != "":
		v.Icon, v.Tooltip = Syncing, "wtfsync — "+v.Busy+"…"
		return
	}
	v.Icon = Idle
	var issues []string
	for _, f := range v.Flavors {
		v.Missing += f.missing()
		text, state := f.issue()
		if text == "" {
			continue
		}
		issues = append(issues, f.Name+": "+text)
		v.Icon = max(v.Icon, state)
	}
	switch len(issues) {
	case 0:
		v.Tooltip = "wtfsync — in sync"
	case 1:
		v.Tooltip = issues[0]
	default:
		v.Tooltip = fmt.Sprintf("%s (+%d more)", issues[0], len(issues)-1)
	}
	if r := []rune(v.Tooltip); len(r) > tooltipMax {
		v.Tooltip = string(r[:tooltipMax-1]) + "…"
	}
}

// MenuItem is one tray menu entry. ID "-" is a separator.
type MenuItem struct {
	ID      string
	Label   string
	Enabled bool
}

// Menu builds the tray menu for v. ui maps IDs to actions.
func Menu(v View) []MenuItem {
	if !v.SetUp {
		return []MenuItem{{"setup", "Set up wtfsync…", v.Busy == ""}, {"-", "", false}, {"quit", "Quit", true}}
	}
	idle := v.Busy == ""
	m := []MenuItem{
		{"push", "Push", idle},
		{"pull", "Pull", idle},
		{"-", "", false},
		{"status", "Status…", true},
		{"restore", "Restore last backup…", idle},
	}
	if v.Missing > 0 {
		m = append(m, MenuItem{"addons", fmt.Sprintf("Missing addons (%d)…", v.Missing), true})
	}
	m = append(m, MenuItem{"settings", "Settings…", idle})
	if v.Update != "" {
		m = append(m, MenuItem{"update", "Update to " + v.Update, idle})
	}
	return append(m, MenuItem{"-", "", false}, MenuItem{"quit", "Quit", true})
}

// friendly turns an engine error into the words a notification shows.
func friendly(flavor string, err error) string {
	switch {
	case errors.Is(err, engine.ErrGameRunning):
		return fmt.Sprintf("Close WoW (%s) first", flavor)
	case errors.Is(err, engine.ErrAborted):
		return "Conflict not resolved; nothing changed"
	}
	return err.Error()
}
```

`internal/app/present.go`:
```go
package app

import (
	"fmt"
	"strings"

	"github.com/rhwendt/wtfsync/internal/setup"
)

// StatusRow is one line of the Status window.
type StatusRow struct {
	Flavor string
	State  string // "in sync", or what's wrong
	Base   string // last synced snapshot, "never synced"
	Newest string // "<snapshot> from <PC>", or "none"
	Local  string // "2 local change(s)"
}

func StatusRows(v View) []StatusRow {
	var rows []StatusRow
	for _, f := range v.Flavors {
		s := f.Status
		r := StatusRow{Flavor: f.Name, State: "in sync", Base: s.Base, Newest: "none", Local: fmt.Sprintf("%d local change(s)", s.LocalChanges)}
		if text, _ := f.issue(); text != "" {
			r.State = text
		}
		if r.Base == "" {
			r.Base = "never synced"
		}
		if s.Newest != "" {
			r.Newest = s.Newest + " from " + s.NewestBy
		}
		rows = append(rows, r)
	}
	return rows
}

// AddonRow is one missing addon in the Missing addons window.
type AddonRow struct {
	Flavor  string
	Machine string // the PC that has it
	Name    string
	Folders string // "ElvUI, ElvUI_Options"
	URL     string // "" when unknown
}

func AddonRows(v View) []AddonRow {
	var rows []AddonRow
	for _, f := range v.Flavors {
		for _, o := range f.Status.MissingAddons {
			for _, m := range o.Addons {
				rows = append(rows, AddonRow{Flavor: f.Name, Machine: o.Machine, Name: m.Name, Folders: strings.Join(m.Folders, ", "), URL: m.URL})
			}
		}
	}
	return rows
}

// ResultLines describes a setup run for the wizard's last page.
func ResultLines(r setup.Result) []string {
	var lines []string
	if r.Rerun && !r.Moved {
		lines = append(lines, "Settings saved; sync history kept.")
	}
	for _, f := range r.Synced {
		rep := f.Report
		switch {
		case f.Err != nil:
			lines = append(lines, fmt.Sprintf("%s: %s", f.Flavor, friendly(f.Flavor, f.Err)))
		case rep.Snapshot != "":
			lines = append(lines, fmt.Sprintf("%s: uploaded %d file(s)", f.Flavor, rep.Pushed))
		case rep.Merged != "":
			lines = append(lines, fmt.Sprintf("%s: got %s's settings (%d file(s))", f.Flavor, rep.From, len(rep.Items)))
		default:
			lines = append(lines, f.Flavor+": nothing to do")
		}
	}
	return lines
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat: add the tray's view model: icon state, tooltip, menu and window text"
```

---

### Task 8: Tray logic: actions, tick, settings, updates

**Files:**
- Create: `internal/app/app.go`
- Test: `internal/app/helpers_test.go`, `internal/app/app_test.go`

**Interfaces:**
- Consumes: Tasks 1, 3, 5, 6, 7; `engine.Engine` (`Push`, `Pull`, `Status`, `Restore`, `RestoreReport`), `local.Store`, `wow.Flavors`, `syncdir.Open`.
- Produces:
  - `var app.ErrBusy`; `type app.Notice struct { ID, Title, Body, Action string }` (Action `"pull"`, `"addons"` or `""`)
  - `type app.Updates interface { Latest(ctx) (update.Release, error); Download(ctx, update.Release, dir string) (string, error) }` (`update.Checker` satisfies it)
  - `type app.Deps struct { Store local.Store; Procs wow.Processes; Hydrator syncdir.Hydrator; Prompt engine.Prompter; Now func() time.Time; Updates Updates; Autostart autostart.Setting; Version string; Notify func(Notice); Changed func(); Log *log.Logger }`
  - `func app.New(Deps) *App`; methods `View() View`, `Settings() (local.Settings, bool, error)`, `Refresh()`, `Tick()`, `Push() error`, `Pull() error`, `RestorePreview(flavor) (engine.RestoreReport, error)`, `Restore(flavor) (engine.RestoreReport, error)`, `Setup(setup.Answers) (setup.Result, error)`, `SaveSettings(SettingsInput) (setup.Result, error)`, `CheckUpdate(ctx)`, `InstallUpdate(ctx, dir string, run func(installer string) error) error`, `HasSyncData(folder) bool`, `Version() string`, `StartsAtLogin() bool`
  - `type app.SettingsInput struct { Name, WoWDir, SyncFolder string; Flavors map[string]bool; StartAtLogin bool }`

- [ ] **Step 1: Write the test helpers** — `internal/app/helpers_test.go`

```go
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
```

- [ ] **Step 2: Write the failing tests** — `internal/app/app_test.go`

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/update"
)

func ids(m []MenuItem) []string {
	var out []string
	for _, it := range m {
		out = append(out, it.ID)
	}
	return out
}

func TestNotSetUpOffersSetupOnly(t *testing.T) {
	_, ny := newPair(t)
	a := ny.app()
	a.Refresh()
	v := a.View()
	if v.SetUp || v.Icon != Attention || v.Tooltip != "wtfsync — not set up" {
		t.Fatalf("view %+v", v)
	}
	if got := ids(Menu(v)); !reflect.DeepEqual(got, []string{"setup", "-", "quit"}) {
		t.Fatalf("menu %v", got)
	}
}

func TestInSyncAfterSetup(t *testing.T) {
	_, _, _, nyApp := linked(t)
	v := nyApp.View()
	if !v.SetUp || v.Icon != Idle || v.Tooltip != "wtfsync — in sync" || len(v.Flavors) != 1 {
		t.Fatalf("view %+v", v)
	}
	want := []string{"push", "pull", "-", "status", "restore", "settings", "-", "quit"}
	if got := ids(Menu(v)); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu %v, want %v", got, want)
	}
}

func TestTickAnnouncesNewSettingsOnce(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	if err := njApp.Push(); err != nil {
		t.Fatal(err)
	}
	nyApp.Tick()
	nyApp.Tick()
	if got := ny.notices.ids(); !reflect.DeepEqual(got, []string{"new-retail"}) {
		t.Fatalf("notices %v, want one new-retail", got)
	}
	n := ny.notices.find(t, "new-retail")
	if n.Action != "pull" || n.Body != "NJ pushed new settings." {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Icon != NewSettings || v.Tooltip != "retail: NJ has newer settings" {
		t.Fatalf("view %+v", v)
	}
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a-nj")
	if v := nyApp.View(); v.Icon != Idle {
		t.Fatalf("after pull: %+v", v)
	}
}

func TestPullAnnouncesMissingAddonsAndMenuListsThem(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.install("Plater", "## X-Curse-Project-ID: 100547")
	nj.write("Account/123#1/SavedVariables/Plater.lua", "p")
	njApp.Push()
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	if n := ny.notices.find(t, "addons"); n.Action != "addons" {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Missing != 1 || v.Icon != Attention {
		t.Fatalf("view %+v", v)
	}
	contains(t, v.Tooltip, "1 addon(s) missing here")
	found := false
	for _, it := range Menu(v) {
		found = found || (it.ID == "addons" && it.Label == "Missing addons (1)…")
	}
	if !found {
		t.Fatalf("menu %+v", Menu(v))
	}
	nyApp.Pull() // same list again: no second notice
	if n := len(ny.notices.ids()); n != 1 {
		t.Fatalf("notices %v", ny.notices.ids())
	}
}

func TestGameRunningRefusesAndExplains(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	ny.procs.running = true
	if err := nyApp.Pull(); !errors.Is(err, engine.ErrGameRunning) {
		t.Fatalf("err = %v", err)
	}
	ny.want(svA, "a0")
	if n := ny.notices.find(t, "error-retail"); n.Body != "Close WoW (retail) first" {
		t.Fatalf("notice %+v", n)
	}
	v := nyApp.View()
	if v.Icon != Attention || v.Tooltip != "retail: Close WoW (retail) first" {
		t.Fatalf("view %+v", v)
	}
	ny.procs.running = false
	nyApp.Pull()
	if v := nyApp.View(); v.Icon != Idle || v.Flavors[0].Err != "" {
		t.Fatalf("error not cleared by a good pull: %+v", v)
	}
}

func TestSyncAlreadyRunningElsewhere(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	held, err := lock.Acquire(ny.store.Dir) // the CLI is pulling
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := nyApp.Pull(); !errors.Is(err, lock.ErrHeld) {
		t.Fatalf("err = %v", err)
	}
	if n := ny.notices.find(t, "busy"); n.Body != "A sync is already running." {
		t.Fatalf("notice %+v", n)
	}
	ny.want(svA, "a0")
}

func TestCancelledConflictChangesNothing(t *testing.T) {
	nj, ny, njApp, _ := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	ny.write(svA, "a-ny")
	ny.prompt = prompt{err: engine.ErrAborted}
	nyApp := ny.app() // picks up the cancelling prompt
	if err := nyApp.Pull(); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
	ny.want(svA, "a-ny")
	if n := ny.notices.find(t, "error-retail"); n.Body != "Conflict not resolved; nothing changed" {
		t.Fatalf("notice %+v", n)
	}
}

func TestTickRetriesPendingPush(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	parked := ny.sync + ".away"
	if err := os.Rename(ny.sync, parked); err != nil { // Drive is offline
		t.Fatal(err)
	}
	ny.write(svA, "a-ny")
	nyApp.Push()
	if v := nyApp.View(); v.Icon != Attention || !v.Flavors[0].Status.PendingPush {
		t.Fatalf("offline push: %+v", v)
	}
	os.Rename(parked, ny.sync)
	nyApp.Tick()
	if st, _ := ny.store.LoadState("retail"); st.PendingPush {
		t.Fatal("pending push not retried")
	}
	if v := nyApp.View(); v.Icon != Idle {
		t.Fatalf("after retry: %+v", v)
	}
}

func TestRestoreOneFlavor(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	njApp.Push()
	nyApp.Pull()
	prev, err := nyApp.RestorePreview("retail")
	if err != nil || !reflect.DeepEqual(prev.Paths, []string{svA}) || prev.Undo != "" {
		t.Fatalf("preview %+v, %v", prev, err)
	}
	ny.want(svA, "a-nj") // preview changes nothing
	rep, err := nyApp.Restore("retail")
	if err != nil || rep.Undo == "" {
		t.Fatalf("restore %+v, %v", rep, err)
	}
	ny.want(svA, "a0")
}

func TestSaveSettingsRenamesTogglesAndStartsAtLogin(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	before, _, _ := nyApp.Settings()
	res, err := nyApp.SaveSettings(SettingsInput{Name: "NY laptop", WoWDir: ny.wowDir, SyncFolder: ny.sync,
		Flavors: map[string]bool{"retail": false}, StartAtLogin: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved || res.Settings.MachineUUID != before.MachineUUID || res.Settings.MachineName != "NY laptop" || res.Settings.Flavors["retail"] {
		t.Fatalf("result %+v", res)
	}
	if !ny.auto.on || ny.auto.sets != 1 {
		t.Fatalf("autostart %+v", ny.auto)
	}
	if v := nyApp.View(); len(v.Flavors) != 0 {
		t.Fatalf("disabled flavor still shown: %+v", v.Flavors)
	}
	nyApp.SaveSettings(SettingsInput{Name: "NY laptop", WoWDir: ny.wowDir, SyncFolder: ny.sync, StartAtLogin: true})
	if ny.auto.sets != 1 {
		t.Fatal("autostart rewritten although unchanged")
	}
}

func TestSaveSettingsNewEmptyFolderStartsItNewFolderWithDataJoinsIt(t *testing.T) {
	nj, ny, _, nyApp := linked(t)
	fresh := t.TempDir()
	res, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: fresh})
	if err != nil || !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot == "" {
		t.Fatalf("empty folder: %+v, %v", res, err) // NY started it and pushed
	}
	res, err = nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: nj.sync})
	if err != nil || !res.Moved || len(res.Synced) != 1 || res.Synced[0].Report.Snapshot != "" {
		t.Fatalf("folder with data: %+v, %v", res, err) // NY joined it and pulled
	}
}

func TestUpdateOfferedOnlyWhenNewer(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	for _, tt := range []struct{ latest, want string }{{"v0.5.0", ""}, {"v0.4.9", ""}, {"v0.6.0", "v0.6.0"}} {
		ny.updates.latest = update.Release{Version: tt.latest}
		nyApp.CheckUpdate(context.Background())
		if got := nyApp.View().Update; got != tt.want {
			t.Fatalf("latest %s: offered %q, want %q", tt.latest, got, tt.want)
		}
	}
	last := Menu(nyApp.View())
	if it := last[len(last)-3]; it.ID != "update" || it.Label != "Update to v0.6.0" {
		t.Fatalf("menu %+v", last)
	}
	ny.updates.err = errOffline // offline: keep quiet, keep the offer
	nyApp.CheckUpdate(context.Background())
	if nyApp.View().Update != "v0.6.0" || len(ny.notices.ids()) != 0 {
		t.Fatal("offline check changed the offer or notified")
	}
}

func TestDevBuildNeverOffersUpdates(t *testing.T) {
	_, ny := newPair(t)
	ny.updates.latest = update.Release{Version: "v9.0.0"}
	a := New(Deps{Store: ny.store, Updates: ny.updates, Version: "dev"})
	a.CheckUpdate(context.Background())
	if a.View().Update != "" {
		t.Fatal("dev build offered an update")
	}
}

func TestInstallUpdateRunsVerifiedInstaller(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.latest = update.Release{Version: "v0.6.0"}
	nyApp.CheckUpdate(context.Background())
	var ran string
	dir := t.TempDir()
	if err := nyApp.InstallUpdate(context.Background(), dir, func(p string) error { ran = p; return nil }); err != nil {
		t.Fatal(err)
	}
	if ran != filepath.Join(dir, "wtfsync-setup-v0.6.0.exe") {
		t.Fatalf("ran %q", ran)
	}
}

func TestInstallUpdateFailureRunsNothing(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.latest = update.Release{Version: "v0.6.0"}
	ny.updates.dlErr = update.ErrChecksum
	nyApp.CheckUpdate(context.Background())
	ran := false
	err := nyApp.InstallUpdate(context.Background(), t.TempDir(), func(string) error { ran = true; return nil })
	if !errors.Is(err, update.ErrChecksum) || ran {
		t.Fatalf("err %v, ran %v", err, ran)
	}
	ny.notices.find(t, "update")
	if nyApp.View().Update != "v0.6.0" {
		t.Fatal("offer withdrawn after a failed download")
	}
}

func TestActionWhileBusyIsRefused(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling")
	if err := nyApp.Push(); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
	nyApp.end()
}

func TestTicksAndActionsAtOnce(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	done := make(chan struct{})
	go func() {
		for range 20 {
			nyApp.Tick()
		}
		close(done)
	}()
	for i := range 10 {
		ny.write(svA, fmt.Sprintf("a%d", i))
		if err := nyApp.Push(); err != nil && !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
	}
	<-done
	if v := nyApp.View(); v.Busy != "" {
		t.Fatalf("left busy: %+v", v)
	}
}

func TestHasSyncData(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if !nyApp.HasSyncData(ny.sync) || nyApp.HasSyncData(t.TempDir()) {
		t.Fatal("HasSyncData")
	}
}

func TestStartsAtLoginAndVersion(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if nyApp.StartsAtLogin() || nyApp.Version() != "v0.5.0" {
		t.Fatal("defaults")
	}
	ny.auto.on = true
	if !nyApp.StartsAtLogin() {
		t.Fatal("StartsAtLogin")
	}
	if New(Deps{}).StartsAtLogin() {
		t.Fatal("no autostart setting should read as off")
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/app/`
Expected: FAIL — `undefined: New`, `undefined: Deps`, `undefined: Notice`.

- [ ] **Step 4: Implement** — `internal/app/app.go`

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/rhwendt/wtfsync/internal/autostart"
	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/lock"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/update"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// ErrBusy means an action is already running in this tray.
var ErrBusy = errors.New("wtfsync is busy")

// Notice is a notification (Windows toast).
type Notice struct {
	ID     string // one per kind of event and flavor, so a newer one replaces an older one
	Title  string
	Body   string
	Action string // what clicking it does: "pull", "addons", or "" (nothing)
}

// Updates is where new releases come from (update.Checker in the real app).
type Updates interface {
	Latest(ctx context.Context) (update.Release, error)
	Download(ctx context.Context, r update.Release, dir string) (string, error)
}

// Deps are the outside world: cmd/wtfsync-tray passes real ones, tests pass fakes.
type Deps struct {
	Store     local.Store
	Procs     wow.Processes
	Hydrator  syncdir.Hydrator
	Prompt    engine.Prompter // the conflict dialog
	Now       func() time.Time
	Updates   Updates
	Autostart autostart.Setting
	Version   string
	Notify    func(Notice) // may be nil
	Changed   func()       // called after the View changes; may be nil
	Log       *log.Logger  // may be nil
}

type App struct {
	d Deps

	act sync.Mutex // held by the running action or tick

	mu       sync.Mutex
	busy     string
	flavors  map[string]*FlavorView
	update   *update.Release
	notified map[string]string // flavor → newest snapshot already announced
	missing  int               // missing addons already announced
}

func New(d Deps) *App {
	if d.Notify == nil {
		d.Notify = func(Notice) {}
	}
	if d.Changed == nil {
		d.Changed = func() {}
	}
	if d.Log == nil {
		d.Log = log.New(io.Discard, "", 0)
	}
	return &App{d: d, flavors: map[string]*FlavorView{}, notified: map[string]string{}}
}

// View is a snapshot of the tray's state.
func (a *App) View() View {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.d.Store.LoadSettings()
	v := View{SetUp: err == nil, Busy: a.busy}
	for _, name := range slices.Sorted(maps.Keys(a.flavors)) {
		v.Flavors = append(v.Flavors, *a.flavors[name])
	}
	if a.update != nil {
		v.Update = a.update.Version
	}
	summarize(&v)
	return v
}

// Settings returns this PC's settings; ok is false when it isn't set up yet.
func (a *App) Settings() (s local.Settings, ok bool, err error) {
	s, err = a.d.Store.LoadSettings()
	if errors.Is(err, local.ErrNoSettings) {
		return s, false, nil
	}
	return s, err == nil, err
}

// enabled loads settings, the engine for them, and the enabled flavors.
func (a *App) enabled() (*engine.Engine, []wow.Flavor, error) {
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return nil, nil, err
	}
	all, err := wow.Flavors(s.WoWDir)
	if err != nil {
		return nil, nil, err
	}
	var on []wow.Flavor
	for _, f := range all {
		if enabled, listed := s.Flavors[f.Name]; enabled || !listed {
			on = append(on, f)
		}
	}
	eng := &engine.Engine{Store: a.d.Store, Settings: s, Procs: a.d.Procs, Hydrator: a.d.Hydrator, Prompt: a.d.Prompt, Now: a.d.Now}
	return eng, on, nil
}

// Refresh reloads every enabled flavor's status. Errors from the last action stay until the
// next action on that flavor.
func (a *App) Refresh() {
	eng, flavors, err := a.enabled()
	if err != nil {
		a.mu.Lock()
		a.flavors = map[string]*FlavorView{}
		a.mu.Unlock()
		a.d.Changed()
		return
	}
	fresh := map[string]*FlavorView{}
	for _, f := range flavors {
		st, err := eng.Status(f)
		if err != nil {
			a.d.Log.Printf("status %s: %v", f.Name, err)
		}
		fresh[f.Name] = &FlavorView{Name: f.Name, Status: st}
	}
	a.mu.Lock()
	for name, fv := range fresh {
		if old, ok := a.flavors[name]; ok {
			fv.Err = old.Err
		}
	}
	a.flavors = fresh
	a.mu.Unlock()
	a.d.Changed()
}

// Tick runs every 60 s: retry pushes that couldn't reach the sync folder, refresh, and announce
// newer settings from other PCs (once per snapshot).
func (a *App) Tick() {
	if !a.act.TryLock() {
		return // an action is running; it refreshes when it ends
	}
	if l, err := lock.Acquire(a.d.Store.Dir); err == nil {
		if eng, flavors, err := a.enabled(); err == nil {
			a.retryPending(eng, flavors)
		}
		l.Release()
	}
	a.act.Unlock()
	a.Refresh()
	for _, f := range a.View().Flavors {
		a.mu.Lock()
		announce := f.newer() && a.notified[f.Name] != f.Status.Newest
		if announce {
			a.notified[f.Name] = f.Status.Newest
		}
		a.mu.Unlock()
		if announce {
			a.d.Notify(Notice{ID: "new-" + f.Name, Title: "New settings for " + f.Name,
				Body: f.Status.NewestBy + " pushed new settings.", Action: "pull"})
		}
	}
}

func (a *App) retryPending(eng *engine.Engine, flavors []wow.Flavor) {
	for _, f := range flavors {
		st, err := a.d.Store.LoadState(f.Name)
		if err != nil || !st.PendingPush {
			continue
		}
		if _, err := eng.Push(f, engine.Options{}); err != nil {
			a.d.Log.Printf("retry push %s: %v", f.Name, err)
		}
	}
}

// begin marks the tray busy with what, waiting for a tick in progress. It returns false if
// another action is already running.
func (a *App) begin(what string) bool {
	a.mu.Lock()
	if a.busy != "" {
		a.mu.Unlock()
		return false
	}
	a.busy = what
	a.mu.Unlock()
	a.act.Lock()
	return true
}

func (a *App) end() {
	a.act.Unlock()
	a.mu.Lock()
	a.busy = ""
	a.mu.Unlock()
}

// run does one user action: busy state, the per-PC lock, then fn per enabled flavor (or only the
// flavor named by only). Per-flavor errors are kept on the flavor and raised as notifications.
func (a *App) run(what, only string, fn func(eng *engine.Engine, f wow.Flavor) error) error {
	if !a.begin(what) {
		return ErrBusy
	}
	a.d.Changed()
	defer func() {
		a.end()
		a.Refresh()
	}()
	l, err := lock.Acquire(a.d.Store.Dir)
	if err != nil {
		if errors.Is(err, lock.ErrHeld) {
			a.d.Notify(Notice{ID: "busy", Title: "wtfsync", Body: "A sync is already running."})
		}
		return err
	}
	defer l.Release()
	eng, flavors, err := a.enabled()
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range flavors {
		if only != "" && f.Name != only {
			continue
		}
		err := fn(eng, f)
		a.mu.Lock()
		fv, ok := a.flavors[f.Name]
		if !ok {
			fv = &FlavorView{Name: f.Name}
			a.flavors[f.Name] = fv
		}
		fv.Err = ""
		if err != nil {
			fv.Err = friendly(f.Name, err)
		}
		a.mu.Unlock()
		if err != nil {
			a.d.Log.Printf("%s %s: %v", what, f.Name, err)
			a.d.Notify(Notice{ID: "error-" + f.Name, Title: "wtfsync: " + f.Name, Body: friendly(f.Name, err)})
			errs = append(errs, fmt.Errorf("%s: %w", f.Name, err))
			if errors.Is(err, engine.ErrAborted) {
				break
			}
		}
	}
	return errors.Join(errs...)
}

// Push pushes every enabled flavor.
func (a *App) Push() error {
	return a.run("pushing", "", func(eng *engine.Engine, f wow.Flavor) error {
		_, err := eng.Push(f, engine.Options{})
		return err
	})
}

// Pull pulls every enabled flavor (retrying a pending push first, as the CLI does) and then
// announces addons other PCs have that this one doesn't, when that list has grown.
func (a *App) Pull() error {
	err := a.run("pulling", "", func(eng *engine.Engine, f wow.Flavor) error {
		a.retryPending(eng, []wow.Flavor{f})
		_, err := eng.Pull(f, engine.Options{})
		return err
	})
	v := a.View()
	a.mu.Lock()
	announce := v.Missing > a.missing
	a.missing = v.Missing
	a.mu.Unlock()
	if announce {
		a.d.Notify(Notice{ID: "addons", Title: "Missing addons",
			Body: fmt.Sprintf("Your other PCs have %d addon(s) this PC doesn't.", v.Missing), Action: "addons"})
	}
	return err
}

// RestorePreview lists what restoring flavor's newest backup would change.
func (a *App) RestorePreview(flavor string) (engine.RestoreReport, error) {
	eng, f, err := a.flavor(flavor)
	if err != nil {
		return engine.RestoreReport{}, err
	}
	return eng.Restore(f, "", engine.Options{DryRun: true})
}

// Restore puts flavor's newest backup back (backing up what it replaces, as the CLI does).
func (a *App) Restore(flavor string) (engine.RestoreReport, error) {
	var rep engine.RestoreReport
	err := a.run("restoring", flavor, func(eng *engine.Engine, f wow.Flavor) error {
		var err error
		rep, err = eng.Restore(f, "", engine.Options{})
		return err
	})
	return rep, err
}

func (a *App) flavor(name string) (*engine.Engine, wow.Flavor, error) {
	eng, flavors, err := a.enabled()
	if err != nil {
		return nil, wow.Flavor{}, err
	}
	for _, f := range flavors {
		if f.Name == name {
			return eng, f, nil
		}
	}
	return nil, wow.Flavor{}, fmt.Errorf("no enabled flavor %q", name)
}

// Setup runs the wizard's answers (first setup, or a rerun).
func (a *App) Setup(ans setup.Answers) (setup.Result, error) {
	if !a.begin("setting up") {
		return setup.Result{}, ErrBusy
	}
	a.d.Changed()
	defer func() {
		a.end()
		a.Refresh()
	}()
	l, err := lock.Acquire(a.d.Store.Dir)
	if err != nil {
		return setup.Result{}, err
	}
	defer l.Release()
	return setup.Run(setup.Env{Store: a.d.Store, Procs: a.d.Procs, Hydrator: a.d.Hydrator, Prompt: a.d.Prompt, Now: a.d.Now}, ans)
}

// SettingsInput is the settings window's form.
type SettingsInput struct {
	Name         string
	WoWDir       string
	SyncFolder   string
	Flavors      map[string]bool
	StartAtLogin bool
}

// SaveSettings applies the settings window. Changing folders follows the setup rerun rules; a new
// sync folder that already has wtfsync data is joined (link), otherwise this PC starts it (init).
func (a *App) SaveSettings(in SettingsInput) (setup.Result, error) {
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return setup.Result{}, err
	}
	for name, on := range in.Flavors {
		s.Flavors[name] = on
	}
	if err := a.d.Store.SaveSettings(s); err != nil {
		return setup.Result{}, err
	}
	_, openErr := syncdir.Open(in.SyncFolder, a.d.Hydrator)
	res, err := a.Setup(setup.Answers{WoWDir: in.WoWDir, SyncFolder: in.SyncFolder, Name: in.Name, First: openErr != nil})
	if err != nil {
		return res, err
	}
	if on, aerr := a.d.Autostart.Enabled(); aerr != nil || on != in.StartAtLogin {
		if err := a.d.Autostart.Set(in.StartAtLogin); err != nil {
			return res, fmt.Errorf("start at login: %w", err)
		}
	}
	return res, nil
}

// CheckUpdate looks for a newer release and offers it in the menu. Failures are only logged:
// being offline mustn't nag.
func (a *App) CheckUpdate(ctx context.Context) {
	if a.d.Updates == nil {
		return
	}
	r, err := a.d.Updates.Latest(ctx)
	if err != nil {
		a.d.Log.Printf("check for updates: %v", err)
		return
	}
	a.mu.Lock()
	if update.Newer(a.d.Version, r.Version) {
		a.update = &r
	} else {
		a.update = nil
	}
	a.mu.Unlock()
	a.d.Changed()
}

// InstallUpdate downloads the offered release into dir, waits for any sync to finish, then calls
// run with the installer's path. The caller quits the tray afterwards; the installer restarts it.
func (a *App) InstallUpdate(ctx context.Context, dir string, run func(installer string) error) error {
	a.mu.Lock()
	r := a.update
	a.mu.Unlock()
	if r == nil {
		return errors.New("no update available")
	}
	if !a.begin("updating") {
		return ErrBusy
	}
	a.d.Changed()
	defer func() {
		a.end()
		a.d.Changed()
	}()
	path, err := a.d.Updates.Download(ctx, *r, dir)
	if err != nil {
		a.d.Notify(Notice{ID: "update", Title: "Update failed", Body: err.Error()})
		return err
	}
	l, err := waitLock(ctx, a.d.Store.Dir)
	if err != nil {
		return err
	}
	defer l.Release()
	return run(path)
}

// waitLock waits for a sync in another process (the CLI) to finish.
func waitLock(ctx context.Context, dir string) (*lock.Lock, error) {
	for {
		l, err := lock.Acquire(dir)
		if !errors.Is(err, lock.ErrHeld) {
			return l, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// HasSyncData reports whether folder already holds wtfsync data, so the wizard can suggest
// "Join my other PC" rather than "This is my first PC".
func (a *App) HasSyncData(folder string) bool {
	_, err := syncdir.Open(folder, a.d.Hydrator)
	return err == nil
}

// Version is the running build's version ("dev" for local builds).
func (a *App) Version() string { return a.d.Version }

// StartsAtLogin reports the start-at-login setting; false when it can't be read.
func (a *App) StartsAtLogin() bool {
	if a.d.Autostart == nil {
		return false
	}
	on, err := a.d.Autostart.Enabled()
	return err == nil && on
}
```

- [ ] **Step 5: Run the tests, including the race detector**

Run: `go test ./internal/app/ && go test -race -count=3 ./internal/app/`
Expected: `ok` both times (the tray calls these from several goroutines).

- [ ] **Step 6: Commit**

```bash
git add internal/app/
git commit -m "feat: add tray logic: push/pull/restore, tick, settings and updates"
```

---

### Task 9: Conflict dialog prompter

**Files:**
- Create: `internal/ui/prompter.go` (no build tag: tested on Linux)
- Test: `internal/ui/prompter_test.go`

**Interfaces:**
- Consumes: `engine.BatchPrompter` (Task 2), `engine.Choice`, `engine.ErrAborted`, `engine.PerFile`.
- Produces: `type ui.ConflictRequest struct { Other string; FirstLink bool; Files []string }`; `type prompter struct { open func(window string); … }` implementing `engine.Prompter` and `engine.BatchPrompter`; `(*prompter).request() ConflictRequest`; `(*prompter).reply(choices map[string]string, cancel bool)` (`"remote"`/`"local"`; later replies ignored).

- [ ] **Step 1: Write the failing test** — `internal/ui/prompter_test.go`

```go
package ui

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// dialog fakes the conflict window: when opened, it reads the request and replies.
func dialog(t *testing.T, reply func(p *prompter, req ConflictRequest)) *prompter {
	p := &prompter{}
	p.open = func(window string) {
		if window != "conflicts" {
			t.Errorf("opened %q", window)
		}
		go reply(p, p.request())
	}
	return p
}

func TestResolveReturnsTheDialogsChoices(t *testing.T) {
	var seen ConflictRequest
	p := dialog(t, func(p *prompter, req ConflictRequest) {
		seen = req
		p.reply(map[string]string{"a.lua": "remote", "b.lua": "local", "c.lua": "bogus"}, false)
	})
	got, err := p.Resolve([]string{"a.lua", "b.lua"}, "NJ", true)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]engine.Choice{"a.lua": engine.ChooseRemote, "b.lua": engine.ChooseLocal}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if seen.Other != "NJ" || !seen.FirstLink || len(seen.Files) != 2 {
		t.Fatalf("request %+v", seen)
	}
	if p.request().Files != nil {
		t.Fatal("request still pending after the reply")
	}
}

func TestCancelOrCloseAborts(t *testing.T) {
	p := dialog(t, func(p *prompter, _ ConflictRequest) { p.reply(nil, true) })
	if _, err := p.Resolve([]string{"a.lua"}, "NJ", false); !errors.Is(err, engine.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
}

func TestSecondReplyIsIgnored(t *testing.T) {
	p := dialog(t, func(p *prompter, _ ConflictRequest) {
		p.reply(map[string]string{"a.lua": "local"}, false)
		p.reply(nil, true) // e.g. the window closing right after Apply
	})
	got, err := p.Resolve([]string{"a.lua"}, "NJ", false)
	if err != nil || got["a.lua"] != engine.ChooseLocal {
		t.Fatalf("got %v, %v", got, err)
	}
	p.reply(nil, true) // nothing waiting: harmless
}

func TestPerFileConflictUsesTheDialog(t *testing.T) {
	p := dialog(t, func(p *prompter, _ ConflictRequest) { p.reply(map[string]string{"a.lua": "remote"}, false) })
	if c, err := p.Conflict("a.lua", "NJ"); err != nil || c != engine.ChooseRemote {
		t.Fatalf("got %v, %v", c, err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/ui/`
Expected: FAIL — `undefined: prompter`, `undefined: ConflictRequest`.

- [ ] **Step 3: Implement** — `internal/ui/prompter.go`

```go
package ui

import (
	"sync"

	"github.com/rhwendt/wtfsync/internal/engine"
)

// ConflictRequest is what the conflict dialog shows.
type ConflictRequest struct {
	Other     string   // the other PC's display name
	FirstLink bool     // this PC has never synced the flavor: offer the one-question choice
	Files     []string // conflicting paths, relative to WTF/
}

// prompter is the engine's BatchPrompter: it opens the conflict dialog and waits for the answer.
// The dialog reads the request with request and answers with reply; closing the window cancels.
type prompter struct {
	open func(window string)

	mu      sync.Mutex
	pending *ConflictRequest
	answer  chan map[string]engine.Choice // nil map = cancelled
}

func (p *prompter) Resolve(conflicts []string, other string, firstLink bool) (map[string]engine.Choice, error) {
	ch := make(chan map[string]engine.Choice, 1)
	p.mu.Lock()
	p.pending = &ConflictRequest{Other: other, FirstLink: firstLink, Files: conflicts}
	p.answer = ch
	p.mu.Unlock()
	p.open("conflicts")
	choices := <-ch
	if choices == nil {
		return nil, engine.ErrAborted
	}
	return choices, nil
}

// request is the dialog's current question; Files is empty when nothing is waiting.
func (p *prompter) request() ConflictRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == nil {
		return ConflictRequest{}
	}
	return *p.pending
}

// reply answers the waiting question once: choices maps path → "remote" or "local"; cancel (or
// closing the window) answers nil. Later replies, and replies with nothing waiting, are ignored.
func (p *prompter) reply(choices map[string]string, cancel bool) {
	p.mu.Lock()
	ch := p.answer
	p.pending, p.answer = nil, nil
	p.mu.Unlock()
	if ch == nil {
		return
	}
	if cancel {
		ch <- nil
		return
	}
	out := map[string]engine.Choice{}
	for path, c := range choices {
		switch c {
		case "remote":
			out[path] = engine.ChooseRemote
		case "local":
			out[path] = engine.ChooseLocal
		}
	}
	ch <- out
}

// The engine only calls these when a Prompter isn't a BatchPrompter; they route through the
// same dialog so behaviour can't differ.
func (p *prompter) FirstLink(int, string) (engine.FirstLinkChoice, error) {
	return engine.PerFile, nil
}

func (p *prompter) Conflict(path, other string) (engine.Choice, error) {
	m, err := p.Resolve([]string{path}, other, false)
	if err != nil {
		return 0, err
	}
	c, ok := m[path]
	if !ok {
		return 0, engine.ErrAborted
	}
	return c, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/ui/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add the conflict dialog's prompter"
```

---

### Task 10: Wails tray, windows and pages; `wtfsync-tray`

**Files:**
- Create: `internal/ui/ui_windows.go`, `internal/ui/bridge_windows.go`, `internal/ui/web/{app.js,app.css,wizard.html,conflicts.html,settings.html,status.html,addons.html,restore.html}`, `cmd/wtfsync-tray/main_windows.go`, `cmd/wtfsync-tray/main_other.go`
- Modify: `go.mod`, `go.sum` (add Wails)

**Interfaces:**
- Consumes: everything in `app` (Tasks 7–8), `prompter` (Task 9), `icons`, `applog`, `autostart`, `update.GitHub`, `setup.FindInstalls`/`Flavors`/`Answers`, `syncdir.Candidates`.
- Produces: `func ui.Run(d app.Deps, o ui.Options) error`; `type ui.Options struct { InstallDirs []string; Getenv func(string) string; Hostname func() (string, error); Log *log.Logger }`; the bound `ui.Bridge` methods the pages call: `Defaults`, `Browse`, `HasSyncData`, `Setup`, `Conflicts`, `AnswerConflicts`, `Status`, `Missing`, `OpenURL`, `Settings`, `SaveSettings`, `RestoreInfo`, `Restore`; the `wtfsync-tray.exe` binary.

These files are Wails glue and HTML: there's nothing to unit-test that Tasks 7–9 don't already cover. They're verified by `GOOS=windows go vet`, the Windows build here, `go test ./...` on CI's `windows-latest` (which compiles them natively), and the manual checklist (Task 12).

- [ ] **Step 1: Add Wails**

Run: `go get github.com/wailsapp/wails/v3@v3.0.0-beta.32`
Expected: `go: added github.com/wailsapp/wails/v3 v3.0.0-beta.32`.

- [ ] **Step 2: Write the tray glue**

`internal/ui/ui_windows.go`:
```go
//go:build windows

// Package ui is the tray app's Wails glue: it renders package app's View as a tray icon and
// menu, opens the windows, shows notifications, and binds Bridge for the pages in web/. It holds
// no sync logic.
package ui

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"golang.org/x/sys/windows"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/icons"
)

//go:embed web
var webFiles embed.FS

// Options are what the windows need beyond app.Deps.
type Options struct {
	InstallDirs []string               // where to look for WoW (wow.DefaultInstallDirs)
	Getenv      func(string) string    // for cloud folder detection
	Hostname    func() (string, error) // default PC name
	Log         *log.Logger
}

const uniqueID = "wtfsync-tray-6f2b7c1e"

type UI struct {
	opts   Options
	app    *app.App
	wails  *application.App
	tray   *application.SystemTray
	notes  *notifications.NotificationService
	prompt *prompter
}

// Run starts the tray and blocks until it quits. With --quit in os.Args it asks a running tray
// to quit instead (Wails' single-instance check hands the arguments over and exits this process);
// if none is running there's nothing to do.
func Run(d app.Deps, o Options) error {
	u := &UI{opts: o}
	u.prompt = &prompter{open: u.open}
	d.Prompt, d.Notify, d.Changed, d.Log = u.prompt, u.notify, u.render, o.Log
	u.app = app.New(d)
	u.notes = notifications.New()
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	u.wails = application.New(application.Options{
		Name:        "wtfsync",
		Description: "Syncs World of Warcraft addon settings between your PCs",
		Services:    []application.Service{application.NewService(u.notes), application.NewService(&Bridge{u: u})},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(web)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               uniqueID,
			OnSecondInstanceLaunch: u.secondInstance,
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true}, // the tray keeps running
	})
	if slices.Contains(os.Args[1:], "--quit") {
		return nil // no other tray was running
	}
	u.tray = u.wails.SystemTray.New()
	u.render()
	return u.wails.Run()
}

// loop runs for the tray's lifetime (started by Bridge.ServiceStartup once Wails is up).
func (u *UI) loop(ctx context.Context) {
	u.notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: "pull", Actions: []notifications.NotificationAction{{ID: "pull", Title: "Pull"}}})
	u.notes.RegisterNotificationCategory(notifications.NotificationCategory{ID: "addons", Actions: []notifications.NotificationAction{{ID: "addons", Title: "Show"}}})
	u.notes.OnNotificationResponse(func(r notifications.NotificationResult) {
		if r.Error != nil {
			return
		}
		action := r.Response.ActionIdentifier
		if action == notifications.DefaultActionIdentifier {
			action = r.Response.CategoryID
		}
		u.do(action)
	})
	u.app.Refresh()
	if !u.app.View().SetUp {
		u.open("wizard")
	}
	u.app.CheckUpdate(ctx)
	tick, daily := time.NewTicker(time.Minute), time.NewTicker(24*time.Hour)
	defer tick.Stop()
	defer daily.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			u.app.Tick()
		case <-daily.C:
			u.app.CheckUpdate(ctx)
		}
	}
}

// render shows the current View: icon, tooltip and menu. Safe from any goroutine.
func (u *UI) render() {
	if u.tray == nil {
		return
	}
	v := u.app.View()
	u.tray.SetIcon(map[app.IconState][]byte{
		app.Idle: icons.Idle, app.NewSettings: icons.NewSettings, app.Attention: icons.Attention, app.Syncing: icons.Syncing,
	}[v.Icon])
	u.tray.SetTooltip(v.Tooltip)
	menu := u.wails.NewMenu()
	for _, it := range app.Menu(v) {
		if it.ID == "-" {
			menu.AddSeparator()
			continue
		}
		id := it.ID
		menu.Add(it.Label).SetEnabled(it.Enabled).OnClick(func(*application.Context) { u.do(id) })
	}
	u.tray.SetMenu(menu)
}

// do runs a menu or notification action. Sync work runs off the UI thread.
func (u *UI) do(id string) {
	switch id {
	case "push":
		go u.app.Push()
	case "pull":
		go u.app.Pull()
	case "update":
		go u.installUpdate()
	case "quit":
		u.wails.Quit()
	case "setup":
		u.open("wizard")
	case "status", "restore", "addons", "settings":
		u.open(id)
	}
}

func (u *UI) installUpdate() {
	dir := filepath.Join(os.TempDir(), "wtfsync-update")
	err := u.app.InstallUpdate(context.Background(), dir, func(installer string) error {
		cmd := exec.Command(installer, "/S")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
		return cmd.Start()
	})
	if err == nil {
		u.wails.Quit() // the installer replaces the files and starts the new tray
	}
}

func (u *UI) notify(n app.Notice) {
	opts := notifications.NotificationOptions{ID: n.ID, Title: n.Title, Body: n.Body, CategoryID: n.Action}
	var err error
	if n.Action != "" {
		err = u.notes.SendNotificationWithActions(opts)
	} else {
		err = u.notes.SendNotification(opts)
	}
	if err != nil && u.opts.Log != nil {
		u.opts.Log.Printf("notify %s: %v", n.ID, err)
	}
}

func (u *UI) secondInstance(data application.SecondInstanceData) {
	if slices.Contains(data.Args, "--quit") {
		u.wails.Quit()
		return
	}
	if u.app.View().SetUp {
		u.open("status")
	} else {
		u.open("wizard")
	}
}

type page struct {
	title         string
	width, height int
}

var pages = map[string]page{
	"wizard":    {"Set up", 560, 520},
	"conflicts": {"Settings changed on both PCs", 640, 480},
	"settings":  {"Settings", 560, 560},
	"status":    {"Status", 640, 420},
	"addons":    {"Missing addons", 600, 420},
	"restore":   {"Restore last backup", 600, 440},
}

// open shows a window, focusing it if it's already open.
func (u *UI) open(name string) {
	application.InvokeAsync(func() {
		if w, ok := u.wails.Window.GetByName(name); ok {
			w.Show()
			w.Focus()
			return
		}
		p := pages[name]
		w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: name, Title: "wtfsync — " + p.title, Width: p.width, Height: p.height, URL: "/" + name + ".html",
		})
		if name == "conflicts" {
			w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.prompt.reply(nil, true) })
		}
		w.Show()
	})
}
```

`internal/ui/bridge_windows.go`:
```go
//go:build windows

package ui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/fsutil"
	"github.com/rhwendt/wtfsync/internal/setup"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// Bridge is the Go side of the pages: each exported method is callable from JavaScript as
// "github.com/rhwendt/wtfsync/internal/ui.Bridge.<Method>" (see web/app.js).
type Bridge struct {
	u *UI
}

func (b *Bridge) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go b.u.loop(ctx)
	return nil
}

// WizardDefaults pre-fills the setup wizard.
type WizardDefaults struct {
	Installs []string // WoW installs found by the drive scan
	Clouds   []string // detected cloud folders
	WoWDir   string
	Sync     string
	Name     string
	Rerun    bool
}

func (b *Bridge) Defaults() WizardDefaults {
	o := b.u.opts
	d := WizardDefaults{Installs: setup.FindInstalls(o.InstallDirs), Clouds: syncdir.Candidates(o.Getenv, fsutil.IsDir)}
	if s, ok, _ := b.u.app.Settings(); ok {
		d.WoWDir, d.Sync, d.Name, d.Rerun = s.WoWDir, s.SyncFolder, s.MachineName, true
	} else {
		d.Name, _ = o.Hostname()
		if len(d.Installs) > 0 {
			d.WoWDir = d.Installs[0]
		}
		if len(d.Clouds) > 0 {
			d.Sync = d.Clouds[0]
		}
	}
	return d
}

// Browse asks for a folder; "" if the user cancels.
func (b *Bridge) Browse(title string) (string, error) {
	return b.u.wails.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).SetTitle(title).PromptForSingleSelection()
}

func (b *Bridge) HasSyncData(folder string) bool { return b.u.app.HasSyncData(folder) }

// Setup runs the wizard. Setup-level failures are errors; per-flavor results are lines.
func (b *Bridge) Setup(a setup.Answers) ([]string, error) {
	res, err := b.u.app.Setup(a)
	switch {
	case errors.Is(err, syncdir.ErrAlreadyInitialized):
		return nil, errors.New(`this cloud folder already has wtfsync data: choose "Join my other PC"`)
	case errors.Is(err, syncdir.ErrNotInitialized):
		return nil, errors.New(`no wtfsync data in this cloud folder yet: set up your first PC, or choose "This is my first PC"`)
	case res.Settings.MachineUUID == "" && err != nil:
		return nil, err
	}
	return app.ResultLines(res), nil
}

func (b *Bridge) Conflicts() ConflictRequest { return b.u.prompt.request() }

// AnswerConflicts settles the waiting conflicts: path → "remote" or "local".
func (b *Bridge) AnswerConflicts(choices map[string]string, cancel bool) {
	b.u.prompt.reply(choices, cancel)
}

type StatusPage struct {
	Rows    []app.StatusRow
	Version string
}

func (b *Bridge) Status() StatusPage {
	b.u.app.Refresh()
	return StatusPage{Rows: app.StatusRows(b.u.app.View()), Version: b.u.app.Version()}
}

func (b *Bridge) Missing() []app.AddonRow { return app.AddonRows(b.u.app.View()) }

// OpenURL opens an addon's download page; only https links are followed.
func (b *Bridge) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("not opening %q", url)
	}
	return b.u.wails.Browser.OpenURL(url)
}

type FlavorToggle struct {
	Name string
	On   bool
}

type SettingsForm struct {
	Name         string
	WoWDir       string
	SyncFolder   string
	Flavors      []FlavorToggle
	StartAtLogin bool
}

func (b *Bridge) Settings() (SettingsForm, error) {
	s, ok, err := b.u.app.Settings()
	if err != nil || !ok {
		return SettingsForm{}, cmp.Or(err, errors.New("this PC isn't set up yet"))
	}
	f := SettingsForm{Name: s.MachineName, WoWDir: s.WoWDir, SyncFolder: s.SyncFolder, StartAtLogin: b.u.app.StartsAtLogin()}
	flavors, _ := setup.Flavors(s.WoWDir)
	for _, fl := range flavors {
		on, listed := s.Flavors[fl.Name]
		f.Flavors = append(f.Flavors, FlavorToggle{Name: fl.Name, On: on || !listed})
	}
	slices.SortFunc(f.Flavors, func(a, b FlavorToggle) int { return strings.Compare(a.Name, b.Name) })
	return f, nil
}

func (b *Bridge) SaveSettings(f SettingsForm) ([]string, error) {
	in := app.SettingsInput{Name: f.Name, WoWDir: f.WoWDir, SyncFolder: f.SyncFolder, StartAtLogin: f.StartAtLogin, Flavors: map[string]bool{}}
	for _, t := range f.Flavors {
		in.Flavors[t.Name] = t.On
	}
	res, err := b.u.app.SaveSettings(in)
	if res.Settings.MachineUUID == "" && err != nil {
		return nil, err
	}
	lines := app.ResultLines(res)
	if err != nil {
		lines = append(lines, err.Error())
	}
	return lines, nil
}

type RestoreInfo struct {
	Flavor string
	Paths  []string
	Err    string // e.g. "no backups for retail yet"
}

func (b *Bridge) RestoreInfo() []RestoreInfo {
	var out []RestoreInfo
	for _, f := range b.u.app.View().Flavors {
		r, err := b.u.app.RestorePreview(f.Name)
		info := RestoreInfo{Flavor: f.Name, Paths: r.Paths}
		if err != nil {
			info.Err = err.Error()
		}
		out = append(out, info)
	}
	return out
}

func (b *Bridge) Restore(flavor string) (string, error) {
	r, err := b.u.app.Restore(flavor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Restored %d file(s). The files it replaced were backed up (%s).", len(r.Paths), r.Undo), nil
}
```

- [ ] **Step 3: Write the pages**

`internal/ui/web/app.js`:
```js
// Shared helpers for wtfsync's windows. Go methods live on ui.Bridge (internal/ui/bridge_windows.go).
import { Call, Window, Browser } from "/wails/runtime.js";

const bridge = "github.com/rhwendt/wtfsync/internal/ui.Bridge.";

export const call = (method, ...args) => Call.ByName(bridge + method, ...args);
export const close = () => Window.Close();
export const openURL = (url) => call("OpenURL", url);

// h builds an element: h("p", {class: "x"}, "text", child, ...). Strings become text nodes.
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false && v != null) el.setAttribute(k, v);
  }
  for (const c of children.flat()) el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  return el;
}

export const $ = (sel) => document.querySelector(sel);

// show replaces #main's content.
export function show(...children) {
  $("#main").replaceChildren(...children);
}

export function errorText(e) {
  return (e && (e.message || e.toString())) || "Something went wrong";
}

// busy disables a button while fn runs, then shows its result or error in #msg.
export async function busy(button, fn) {
  button.disabled = true;
  $("#msg").textContent = "";
  try {
    return await fn();
  } catch (e) {
    $("#msg").textContent = errorText(e);
    $("#msg").className = "error";
  } finally {
    button.disabled = false;
  }
}

// folderPicker is a <select> of options plus "Browse…"; returns {el, value()}.
export function folderPicker(options, current, title) {
  const sel = h("select", {});
  const add = (v) => { if (v && ![...sel.options].some((o) => o.value === v)) sel.append(h("option", { value: v }, v)); };
  options.forEach(add);
  add(current);
  sel.value = current || options[0] || "";
  const browse = h("button", { type: "button", onclick: async () => {
    const dir = await call("Browse", title);
    if (dir) { add(dir); sel.value = dir; sel.dispatchEvent(new Event("change")); }
  } }, "Browse…");
  return { el: h("div", { class: "row" }, sel, browse), select: sel, value: () => sel.value };
}
```

`internal/ui/web/app.css`:
```css
:root { color-scheme: light dark; --accent: #2f7fd8; --muted: #888; --bad: #d63b3b; }
body { font: 14px/1.45 "Segoe UI", system-ui, sans-serif; margin: 0; padding: 20px 24px; }
h1 { font-size: 18px; margin: 0 0 12px; }
label { display: block; margin: 14px 0 4px; font-weight: 600; }
.row { display: flex; gap: 8px; }
select, input[type=text] { flex: 1; padding: 5px; font: inherit; min-width: 0; }
button { font: inherit; padding: 5px 14px; }
button.primary { background: var(--accent); color: white; border: 1px solid var(--accent); border-radius: 4px; }
.actions { margin-top: 20px; display: flex; gap: 8px; justify-content: flex-end; }
.muted { color: var(--muted); }
.error { color: var(--bad); }
#msg { margin-top: 12px; white-space: pre-wrap; }
table { border-collapse: collapse; width: 100%; }
td, th { text-align: left; padding: 4px 8px 4px 0; vertical-align: top; }
th { font-weight: 600; }
code { font-size: 12px; word-break: break-all; }
ul.lines { padding-left: 18px; }
```

`internal/ui/web/wizard.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Set up wtfsync</title></head>
<body><h1>Set up wtfsync</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show, busy, folderPicker } from "./app.js";

const d = await call("Defaults");
const wow = folderPicker(d.Installs, d.WoWDir, "Your World of Warcraft folder");
const cloud = folderPicker(d.Clouds, d.Sync, "A folder your cloud client syncs");
const name = h("input", { type: "text", value: d.Name });
const first = h("input", { type: "radio", name: "mode", id: "first" });
const join = h("input", { type: "radio", name: "mode", id: "join" });

// Suggest joining when the chosen cloud folder already has wtfsync data.
async function suggest() {
  const has = await call("HasSyncData", cloud.value());
  (has ? join : first).checked = true;
}
cloud.select.addEventListener("change", suggest);
await suggest();

const go = h("button", { class: "primary", onclick: () => busy(go, async () => {
  const lines = await call("Setup", { WoWDir: wow.value(), SyncFolder: cloud.value(), Name: name.value, First: first.checked });
  show(h("p", {}, d.Rerun ? "Saved." : "This PC is set up."), h("ul", { class: "lines" }, lines.map((l) => h("li", {}, l))),
    h("div", { class: "actions" }, h("button", { class: "primary", onclick: close }, "Done")));
}) }, d.Rerun ? "Save" : "Set up");

show(
  h("label", {}, "World of Warcraft folder"), wow.el,
  h("label", {}, "Cloud folder (Google Drive, OneDrive, Dropbox…)"), cloud.el,
  h("label", {}, "Name for this PC"), h("div", { class: "row" }, name),
  h("label", {}, "This PC is…"),
  h("div", {}, first, h("label", { for: "first", style: "display:inline;font-weight:normal" }, " my first PC (upload its settings)")),
  h("div", {}, join, h("label", { for: "join", style: "display:inline;font-weight:normal" }, " joining my other PC (download its settings)")),
  h("div", { class: "actions" }, h("button", { onclick: close }, "Cancel"), go),
);
</script></body></html>
```

`internal/ui/web/conflicts.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Conflicts</title></head>
<body><h1>Settings changed on both PCs</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show } from "./app.js";

const req = await call("Conflicts");
const answer = (choices) => { call("AnswerConflicts", choices, false); close(); };
const all = (c) => Object.fromEntries(req.Files.map((f) => [f, c]));

function perFile() {
  const rows = req.Files.map((f) => {
    const sel = h("select", {}, h("option", { value: "remote" }, `keep ${req.Other}'s`), h("option", { value: "local" }, "keep this PC's"));
    return { f, sel, tr: h("tr", {}, h("td", {}, h("code", {}, f)), h("td", {}, sel)) };
  });
  const setAll = (c) => rows.forEach((r) => (r.sel.value = c));
  show(
    h("p", {}, `These files changed on ${req.Other} and on this PC since they last synced. Choose which to keep:`),
    h("table", {}, rows.map((r) => r.tr)),
    h("div", { class: "actions" },
      h("button", { onclick: () => setAll("remote") }, `All ${req.Other}'s`),
      h("button", { onclick: () => setAll("local") }, "All this PC's"),
      h("button", { onclick: () => { call("AnswerConflicts", null, true); close(); } }, "Cancel"),
      h("button", { class: "primary", onclick: () => answer(Object.fromEntries(rows.map((r) => [r.f, r.sel.value]))) }, "Apply")),
  );
}

if (!req.Files || req.Files.length === 0) {
  show(h("p", { class: "muted" }, "Nothing to decide."), h("div", { class: "actions" }, h("button", { onclick: close }, "Close")));
} else if (req.FirstLink) {
  show(
    h("p", {}, `This PC and ${req.Other} both have settings for ${req.Files.length} file(s).`),
    h("div", { class: "actions" },
      h("button", { onclick: () => { call("AnswerConflicts", null, true); close(); } }, "Cancel"),
      h("button", { onclick: perFile }, "Choose per file…"),
      h("button", { onclick: () => answer(all("local")) }, "Keep this PC's"),
      h("button", { class: "primary", onclick: () => answer(all("remote")) }, `Use ${req.Other}'s for all`)),
  );
} else {
  perFile();
}
</script></body></html>
```

`internal/ui/web/settings.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Settings</title></head>
<body><h1>Settings</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show, busy, folderPicker, errorText, $ } from "./app.js";

let s;
try { s = await call("Settings"); } catch (e) { show(h("p", { class: "error" }, errorText(e))); throw e; }
const d = await call("Defaults");
const name = h("input", { type: "text", value: s.Name });
const wow = folderPicker(d.Installs, s.WoWDir, "Your World of Warcraft folder");
const cloud = folderPicker(d.Clouds, s.SyncFolder, "A folder your cloud client syncs");
const toggles = s.Flavors.map((f) => ({ f, box: h("input", { type: "checkbox", checked: f.On }) }));
const login = h("input", { type: "checkbox", id: "login", checked: s.StartAtLogin });

const save = h("button", { class: "primary", onclick: () => busy(save, async () => {
  const lines = await call("SaveSettings", {
    Name: name.value, WoWDir: wow.value(), SyncFolder: cloud.value(), StartAtLogin: login.checked,
    Flavors: toggles.map((t) => ({ Name: t.f.Name, On: t.box.checked })),
  });
  $("#msg").className = "";
  $("#msg").textContent = lines.join("\n") || "Saved.";
}) }, "Save");

show(
  h("label", {}, "Name for this PC"), h("div", { class: "row" }, name),
  h("label", {}, "World of Warcraft folder"), wow.el,
  h("label", {}, "Cloud folder"), cloud.el,
  h("p", { class: "muted" }, "Changing a folder clears this PC's sync history: the next sync asks once which settings to use."),
  h("label", {}, "Flavors to sync"),
  toggles.map((t) => h("div", {}, t.box, " ", t.f.Name)),
  h("label", {}, "Startup"), h("div", {}, login, h("label", { for: "login", style: "display:inline;font-weight:normal" }, " Start wtfsync when Windows starts")),
  h("div", { class: "actions" }, h("button", { onclick: close }, "Close"), save),
);
</script></body></html>
```

`internal/ui/web/status.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Status</title></head>
<body><h1>Status</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show } from "./app.js";

async function render() {
  const p = await call("Status");
  show(
    p.Rows.length === 0 ? h("p", { class: "muted" }, "No flavors are being synced.") :
    h("table", {}, h("tr", {}, ["Flavor", "State", "Last synced", "Newest", "Here"].map((t) => h("th", {}, t))),
      p.Rows.map((r) => h("tr", {}, h("td", {}, r.Flavor), h("td", {}, r.State), h("td", {}, h("code", {}, r.Base)), h("td", {}, h("code", {}, r.Newest)), h("td", {}, r.Local)))),
    h("p", { class: "muted" }, `wtfsync ${p.Version}`),
    h("div", { class: "actions" }, h("button", { onclick: render }, "Refresh"), h("button", { class: "primary", onclick: close }, "Close")),
  );
}
await render();
</script></body></html>
```

`internal/ui/web/addons.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Missing addons</title></head>
<body><h1>Missing addons</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show, openURL } from "./app.js";

const rows = await call("Missing");
show(
  rows.length === 0 ? h("p", {}, "This PC has every addon your other PCs have.") :
  h("div", {},
    h("p", {}, "Your other PCs have these addons and this PC doesn't. Their settings are already here; install them to use them."),
    h("table", {}, h("tr", {}, ["Addon", "Flavor", "On", ""].map((t) => h("th", {}, t))),
      rows.map((r) => h("tr", {},
        h("td", {}, r.Name, r.Folders !== r.Name ? h("div", { class: "muted" }, r.Folders) : ""),
        h("td", {}, r.Flavor), h("td", {}, r.Machine),
        h("td", {}, r.URL ? h("a", { href: "#", onclick: (e) => { e.preventDefault(); openURL(r.URL); } }, "Download") : h("span", { class: "muted" }, "no link")))))),
  h("div", { class: "actions" }, h("button", { class: "primary", onclick: close }, "Close")),
);
</script></body></html>
```

`internal/ui/web/restore.html`:
```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Restore</title></head>
<body><h1>Restore last backup</h1><div id="main"></div><div id="msg"></div>
<script type="module">
import { call, close, h, show, busy, $ } from "./app.js";

const infos = await call("RestoreInfo");
show(
  h("p", {}, "Puts back the files the last pull replaced. What's there now is backed up first, so this can be undone."),
  infos.map((i) => {
    if (i.Err) return h("p", {}, h("b", {}, i.Flavor), ": ", h("span", { class: "muted" }, i.Err));
    const b = h("button", { onclick: () => busy(b, async () => {
      $("#msg").className = "";
      $("#msg").textContent = await call("Restore", i.Flavor);
    }) }, `Restore ${i.Flavor}`);
    return h("div", {}, h("p", {}, h("b", {}, i.Flavor), `: ${i.Paths.length} file(s)`), h("ul", { class: "lines" }, i.Paths.map((p) => h("li", {}, h("code", {}, p)))), b);
  }),
  h("div", { class: "actions" }, h("button", { class: "primary", onclick: close }, "Close")),
);
</script></body></html>
```

- [ ] **Step 4: Write the entry points**

`cmd/wtfsync-tray/main_windows.go`:
```go
//go:build windows

// Command wtfsync-tray is wtfsync's tray app: a tray icon showing sync state, a menu to push and
// pull, and windows for setup, conflicts and settings. Build with -ldflags "-H windowsgui".
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rhwendt/wtfsync/internal/app"
	"github.com/rhwendt/wtfsync/internal/applog"
	"github.com/rhwendt/wtfsync/internal/autostart"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
	"github.com/rhwendt/wtfsync/internal/ui"
	"github.com/rhwendt/wtfsync/internal/update"
	"github.com/rhwendt/wtfsync/internal/wow"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		// No console in a GUI program: the log file is where this ends up.
		fmt.Fprintln(os.Stderr, "wtfsync-tray:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := os.UserConfigDir() // %APPDATA%
	if err != nil {
		return err
	}
	dataDir := filepath.Join(cfg, "wtfsync")
	logFile, err := applog.Open(filepath.Join(dataDir, "tray.log"), applog.DefaultMax)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logger.Printf("wtfsync-tray %s starting", version)
	err = ui.Run(app.Deps{
		Store:     local.Store{Dir: dataDir},
		Procs:     wow.System(),
		Hydrator:  syncdir.TimeoutHydrator{Timeout: syncdir.DefaultHydrateTimeout},
		Now:       time.Now,
		Updates:   update.GitHub(),
		Autostart: autostart.Run{Name: "wtfsync", Command: `"` + exe + `"`},
		Version:   version,
	}, ui.Options{InstallDirs: wow.DefaultInstallDirs(), Getenv: os.Getenv, Hostname: os.Hostname, Log: logger})
	if err != nil {
		logger.Printf("exit: %v", err)
	}
	return err
}
```

`cmd/wtfsync-tray/main_other.go`:
```go
//go:build !windows

// Command wtfsync-tray is wtfsync's Windows tray app; on other systems it only says so.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "wtfsync-tray runs on Windows; use the wtfsync command here.")
	os.Exit(1)
}
```

- [ ] **Step 5: Tidy, vet both OSes, build the tray**

Run: `go mod tidy && go vet ./... && CGO_ENABLED=0 GOOS=windows go vet ./... && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o /tmp/wtfsync-tray.exe ./cmd/wtfsync-tray && ls -l /tmp/wtfsync-tray.exe && go test ./... && go run ./cmd/wtfsync-tray`
Expected: no vet output; a ~18 MB `wtfsync-tray.exe`; every package `ok`; the last command prints `wtfsync-tray runs on Windows; use the wtfsync command here.` and exits 1.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/ui/ cmd/wtfsync-tray/
git commit -m "feat: add the tray app: icon, menu, windows and notifications"
```

---

### Task 11: Installer, release build, CI

**Files:**
- Create: `installer/wtfsync.nsi`, `.github/scripts/build.sh`
- Modify: `.github/workflows/ci.yml` (new `version` job; `build` builds `dist/`; `release` attaches the installer), `.gitignore` (`/dist/`)

**Interfaces:**
- Consumes: `cmd/wtfsync`, `cmd/wtfsync-tray`, `main.version` (Tasks 4, 10); `.github/scripts/next-version.sh` and `release-notes.py` (already in the repo).
- Produces: `dist/wtfsync.exe`, `dist/wtfsync-tray.exe`, `dist/wtfsync-setup.exe`, `dist/checksums.txt`; release assets `wtfsync-setup.exe`, `wtfsync.exe`, `checksums.txt` (names `update` relies on).

- [ ] **Step 1: Write the installer** — `installer/wtfsync.nsi`

```nsis
; wtfsync installer: per-user (no admin), built on Linux with
;   makensis -NOCD -DVERSION=v1.2.3 -DBINDIR=dist -DOUTFILE=dist/wtfsync-setup.exe installer/wtfsync.nsi
; (-NOCD: paths are relative to the repo root, not this folder.)
; BINDIR must hold wtfsync.exe and wtfsync-tray.exe. /S installs silently (used by the tray's
; one-click update) and keeps the current "start at login" and PATH choices.

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!ifndef VERSION
  !define VERSION dev
!endif
!ifndef BINDIR
  !define BINDIR dist
!endif
!ifndef OUTFILE
  !define OUTFILE wtfsync-setup.exe
!endif

!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\wtfsync"
!define RUNKEY "Software\Microsoft\Windows\CurrentVersion\Run"

Name "wtfsync ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\wtfsync"
InstallDirRegKey HKCU "${UNINSTKEY}" "InstallLocation"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "Sections.nsh"
!include "StrFunc.nsh"
!include "WinMessages.nsh"
${StrStr}
${StrRep}
${UnStrStr}
${UnStrRep}

!define MUI_COMPONENTSPAGE_NODESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\wtfsync-tray.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Start wtfsync now"
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

; Ask the running tray to quit, then wait up to 10 s; force it after that.
!macro StopTray
  IfFileExists "$INSTDIR\wtfsync-tray.exe" 0 +2
    ExecWait '"$INSTDIR\wtfsync-tray.exe" --quit'
  StrCpy $R9 0
  ${Do}
    nsExec::Exec 'cmd /c tasklist /FI "IMAGENAME eq wtfsync-tray.exe" /NH | find /I "wtfsync-tray.exe"'
    Pop $R8
    ${If} $R8 != 0
      ${Break}
    ${EndIf}
    ${If} $R9 >= 20
      nsExec::Exec 'taskkill /F /IM wtfsync-tray.exe'
      Pop $R8
      Sleep 500
      ${Break}
    ${EndIf}
    Sleep 500
    IntOp $R9 $R9 + 1
  ${Loop}
!macroend

Section "wtfsync" SecMain
  SectionIn RO
  !insertmacro StopTray
  SetOutPath "$INSTDIR"
  File "${BINDIR}/wtfsync.exe"
  File "${BINDIR}/wtfsync-tray.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortcut "$SMPROGRAMS\wtfsync.lnk" "$INSTDIR\wtfsync-tray.exe"

  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "wtfsync"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "wtfsync"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\wtfsync-tray.exe"
  WriteRegStr HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "EstimatedSize" 40000
SectionEnd

Section "Start wtfsync when Windows starts" SecAutostart
SectionEnd

Section "Add the wtfsync command to PATH" SecPath
SectionEnd

; Applies the two options (selected or not) after the files are in place.
Section "-apply"
  ${If} ${SectionIsSelected} ${SecAutostart}
    WriteRegStr HKCU "${RUNKEY}" "wtfsync" '"$INSTDIR\wtfsync-tray.exe"'
  ${Else}
    DeleteRegValue HKCU "${RUNKEY}" "wtfsync"
  ${EndIf}

  ReadRegStr $0 HKCU "Environment" "Path"
  ${StrStr} $1 "$0;" "$INSTDIR;"
  ${If} ${SectionIsSelected} ${SecPath}
    ${If} $1 == ""
      ${If} $0 == ""
        WriteRegExpandStr HKCU "Environment" "Path" "$INSTDIR"
      ${Else}
        WriteRegExpandStr HKCU "Environment" "Path" "$0;$INSTDIR"
      ${EndIf}
    ${EndIf}
  ${ElseIf} $1 != ""
    ${StrRep} $0 "$0;" "$INSTDIR;" ""
    StrCpy $0 $0 -1 ; drop the trailing ";" added for the search
    WriteRegExpandStr HKCU "Environment" "Path" "$0"
  ${EndIf}
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  ${If} ${Silent}
    Exec '"$INSTDIR\wtfsync-tray.exe"' ; a silent update restarts the tray itself
  ${EndIf}
SectionEnd

; On an upgrade, start from the current choices instead of the defaults.
Function .onInit
  ReadRegStr $0 HKCU "${UNINSTKEY}" "InstallLocation"
  ${If} $0 != ""
    ReadRegStr $1 HKCU "${RUNKEY}" "wtfsync"
    ${If} $1 == ""
      !insertmacro UnselectSection ${SecAutostart}
    ${EndIf}
    ReadRegStr $1 HKCU "Environment" "Path"
    ${StrStr} $2 "$1;" "$0;"
    ${If} $2 == ""
      !insertmacro UnselectSection ${SecPath}
    ${EndIf}
  ${EndIf}
FunctionEnd

; Removes the programs, shortcut, startup entry and PATH entry. Keeps %APPDATA%\wtfsync
; (settings, sync state, backups) and the data in the cloud folder.
Section "Uninstall"
  !insertmacro StopTray
  Delete "$INSTDIR\wtfsync.exe"
  Delete "$INSTDIR\wtfsync-tray.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\wtfsync.lnk"
  DeleteRegValue HKCU "${RUNKEY}" "wtfsync"
  ReadRegStr $0 HKCU "Environment" "Path"
  ${UnStrStr} $1 "$0;" "$INSTDIR;"
  ${If} $1 != ""
    ${UnStrRep} $0 "$0;" "$INSTDIR;" ""
    StrCpy $0 $0 -1
    WriteRegExpandStr HKCU "Environment" "Path" "$0"
    SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
  ${EndIf}
  DeleteRegKey HKCU "${UNINSTKEY}"
SectionEnd
```

- [ ] **Step 2: Write the build script** — `.github/scripts/build.sh`, then `chmod +x .github/scripts/build.sh`

```bash
#!/usr/bin/env bash
# Builds the Windows release files into dist/: wtfsync.exe (CLI), wtfsync-tray.exe (tray app),
# wtfsync-setup.exe (installer, needs makensis) and checksums.txt.
# usage: build.sh VERSION   (e.g. v0.5.0, or dev)
set -euo pipefail

version=${1:?usage: build.sh VERSION}
ldflags="-s -w -X main.version=$version"
rm -rf dist && mkdir dist
export CGO_ENABLED=0 GOOS=windows GOARCH=amd64
go build -trimpath -ldflags "$ldflags" -o dist/wtfsync.exe ./cmd/wtfsync
go build -trimpath -ldflags "$ldflags -H windowsgui" -o dist/wtfsync-tray.exe ./cmd/wtfsync-tray
makensis -NOCD -V2 -WX -DVERSION="$version" -DBINDIR=dist -DOUTFILE=dist/wtfsync-setup.exe installer/wtfsync.nsi
(cd dist && sha256sum wtfsync.exe wtfsync-tray.exe wtfsync-setup.exe > checksums.txt)
```

- [ ] **Step 3: Build locally**

Run: `.github/scripts/build.sh v0.0.0-test && ls -l dist && cat dist/checksums.txt`
(No local `makensis`? `apt install nsis`, or run makensis from an `ubuntu:24.04` container with the repo mounted.)
Expected: the four files (installer ≈ 5 MB); `checksums.txt` has three `<sha256>  <name>` lines. `makensis -WX` would have failed on any warning.

- [ ] **Step 4: Update CI** — replace `.github/workflows/ci.yml` with:

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...

  # The version is known before building so it's built into both programs: the next release's
  # tag on pushes to main (empty when nothing warrants a release), otherwise "dev".
  version:
    runs-on: ubuntu-latest
    outputs:
      tag: ${{ steps.next.outputs.tag }}
      build: ${{ steps.next.outputs.build }}
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - id: next
        run: |
          tag=""
          if [ "${{ github.event_name }}" = push ] && [ "${{ github.ref }}" = refs/heads/main ]; then
            tag=$(.github/scripts/next-version.sh)
          fi
          echo "tag=$tag" >> "$GITHUB_OUTPUT"
          echo "build=${tag:-dev}" >> "$GITHUB_OUTPUT"

  build:
    needs: version
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: sudo apt-get update -qq && sudo apt-get install -y -qq nsis
      - run: .github/scripts/build.sh "${{ needs.version.outputs.build }}"
      - uses: actions/upload-artifact@v7
        with:
          name: wtfsync-windows-amd64
          path: |
            dist/wtfsync.exe
            dist/wtfsync-tray.exe
            dist/wtfsync-setup.exe
            dist/checksums.txt

  # Every merge to main whose commits include a feat, fix, perf or breaking change publishes a
  # GitHub Release with the installer, the CLI and their checksums, versioned from the
  # Conventional Commits since the last tag.
  release:
    needs: [version, test, build]
    if: needs.version.outputs.tag != ''
    runs-on: ubuntu-latest
    permissions:
      contents: write
    concurrency: release
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - uses: actions/download-artifact@v8
        with:
          name: wtfsync-windows-amd64
          path: dist
      - env:
          GH_TOKEN: ${{ github.token }}
          TAG: ${{ needs.version.outputs.tag }}
        run: |
          .github/scripts/release-notes.py "$TAG" > notes.md
          gh release create "$TAG" dist/wtfsync-setup.exe dist/wtfsync.exe dist/checksums.txt \
            --target "$GITHUB_SHA" --title "$TAG" --notes-file notes.md
```

Check the current major of `actions/upload-artifact` / `download-artifact` / `checkout` / `setup-go` (`gh api repos/actions/<name>/releases/latest --jq .tag_name`) and use it if newer.

- [ ] **Step 5: Ignore `dist/`** — `.gitignore`:

```
/wtfsync.exe
/wtfsync
/dist/
```

- [ ] **Step 6: Commit**

```bash
git add installer/ .github/ .gitignore
git commit -m "feat: build a per-user installer and attach it to releases"
```

---

### Task 12: README, push, PR

**Files:**
- Modify: `README.md` (Download, new Tray app section, Build, manual checklist items 5–12)

**Interfaces:**
- Consumes: everything. Produces: user docs; the PR.

- [ ] **Step 1: Update `README.md`** — the full file:

```markdown
# wtfsync

Sync World of Warcraft addon settings (SavedVariables) between PCs through a folder your cloud
client already syncs (Google Drive for Desktop, OneDrive, iCloud, Dropbox).

It syncs `WTF/Account/**/SavedVariables/*.lua` for every installed flavor (retail, classic, …).
It never touches `Config.wtf` (graphics/hardware settings stay per-PC) or addon code.

It doesn't install addons, but it tells you what you're missing. Each push records the PC's
installed addons, and `pull`, `push` and `status` on the other PC list any it doesn't have, with
a download link when the addon's `.toc` names its CurseForge, Wago or WoWInterface project:

    NJ has addons not installed here:
      Plater  https://www.curseforge.com/projects/100547
      ElvUI (ElvUI, ElvUI_Options)  https://addons.wago.io/addons/…

The settings arrive anyway, so once you install the addon it's already set up.

## Setup

On your first PC:

    wtfsync init

On each other PC (point it at the same cloud folder):

    wtfsync link

To change a PC's WoW folder, cloud folder or name later, run `init` (or `link`) again: your
current answers are the defaults and the PC keeps its identity. Pointing it at a different folder
makes the next sync ask once which settings to use, like a first link.

## Use

Close WoW, then:

    wtfsync push      # after playing
    wtfsync pull      # before playing on the other PC
    wtfsync status    # what's changed, and whether you're behind

If the same addon's settings changed on both PCs, wtfsync asks which one to keep. Every pull backs
up the files it overwrites; `wtfsync restore` puts the last backup back.

Add `--dry-run` to any command to see what it would do without changing anything.

## Download

Run `wtfsync-setup.exe` from the latest [release](https://github.com/rhwendt/wtfsync/releases/latest).
It installs for your Windows user only (no admin prompt) into `%LOCALAPPDATA%\Programs\wtfsync`,
adds a Start menu entry, and can start wtfsync with Windows and add the `wtfsync` command to PATH.
It isn't code-signed yet, so Windows SmartScreen asks once: **More info → Run anyway**. wtfsync's
windows use Microsoft Edge WebView2, which Windows 10 and 11 already include.

The tray app checks for new releases once a day and offers **Update to vX.Y.Z** in its menu.
Uninstalling (Settings → Apps) keeps your settings and backups in `%APPDATA%\wtfsync`.

The release also has `wtfsync.exe` on its own, for using just the command line.

Releases are automatic: every merge to `main` that includes a `feat:`, `fix:`, `perf:` or breaking
change ([Conventional Commits](https://www.conventionalcommits.org/)) publishes a new version. `fix`
bumps the patch number, `feat` the minor, and a breaking change (`feat!:` or a `BREAKING CHANGE:`
footer) the major (the minor while still 0.x). Merges with only `docs:`, `chore:`, `ci:`, `test:` or
`refactor:` commits don't release.

Each release's notes are the changelog: its features, fixes and breaking changes, grouped from the
commit messages, with the PR each came from. See [all releases](https://github.com/rhwendt/wtfsync/releases).

## Tray app

`wtfsync-tray.exe` (the Start menu entry) puts an icon in the notification area:

| Icon | Means |
|---|---|
| green | every flavor is in sync |
| amber | another PC pushed newer settings (a notification offers **Pull**) |
| red | needs you: an error, a refused sync (close WoW first), a push waiting for the cloud folder, or missing addons |
| blue | syncing |

Its menu has Push, Pull, Status, Restore last backup, Missing addons, Settings and Update. A
conflict (the same addon changed on both PCs) opens a window to choose which version to keep;
Cancel changes nothing. The tray and the `wtfsync` command never sync at the same time: whichever
starts second says "a sync is already running". The tray logs to `%APPDATA%\wtfsync\tray.log`.

## Build

    .github/scripts/build.sh dev    # needs makensis (apt install nsis)

builds `dist/wtfsync.exe`, `dist/wtfsync-tray.exe`, `dist/wtfsync-setup.exe` and
`dist/checksums.txt`. CI does the same for every PR (artifact `wtfsync-windows-amd64`).
`wtfsync version` prints the version a build was made with.

## Manual test checklist (real PCs)

1. `wtfsync init --dry-run`: the flavors and file list look right.
2. `push` / `pull` with `--wow-dir` pointed at a *copy* of the install.
3. NJ ↔ NY through Google Drive for Desktop (including streaming/placeholder mode).
4. Change the same addon on both PCs without syncing: the second push asks.
5. Install `wtfsync-setup.exe`: Start menu entry, tray icon, the wizard on a PC that isn't set up.
6. Push on one PC → a notification on the other within a minute → **Pull** from it.
7. Same-file conflict → the conflict window; Cancel leaves files as they were.
8. WoW running → Pull is refused with "Close WoW (flavor) first".
9. Tray syncing while `wtfsync pull` runs in cmd → "a sync is already running".
10. Settings: rename the PC, toggle a flavor, start at login (check after a reboot).
11. Update from one release to the next from the tray menu; the tray comes back on its own.
12. Uninstall: programs, shortcut and startup entry gone; `%APPDATA%\wtfsync` still there.
```

- [ ] **Step 2: Full verification**

Run: `go vet ./... && CGO_ENABLED=0 GOOS=windows go vet ./... && go test ./... && go test -race ./internal/app/ ./internal/ui/ ./internal/lock/ && .github/scripts/build.sh dev`
Expected: no vet output; all packages `ok`; `dist/` has all four files.

- [ ] **Step 3: Commit, push, open the PR, watch CI**

```bash
git add README.md
git commit -m "docs: document the installer, tray app and build"
git push -u origin feature/m2-tray
gh pr create --title "feat: milestone 2 tray app, installer and updates" --body "Implements docs/superpowers/specs/2026-10-19-m2-tray-installer-design.md (plan: docs/superpowers/plans/2026-10-19-m2-tray-installer.md). Manual checklist: README items 5–12.

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
```

Then `gh run watch` the PR's CI: `test (ubuntu-latest)`, `test (windows-latest)` (compiles the Wails code natively and runs the autostart/lock Windows tests) and `build` (installer) must pass. Download the `wtfsync-windows-amd64` artifact and run the manual checklist on a real PC before merging.

---

## After M2

M3 (automation: auto-push on game exit, Play button with session markers, Battle.net launch) gets its own plan after M2 is merged and used on both PCs. Before writing it, confirm the Battle.net launch codes and the `wow*.exe` names per flavor on the real install.
