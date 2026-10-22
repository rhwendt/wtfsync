# Main Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the tray's five bare windows with one designed main window (flavor list on the left, the selected flavor's page on the right), with backups you can restore by choice, a light/dark/system appearance setting, an All flavors switch, and a README that shows the app and documents every CLI command.

**Architecture:** As in M2, `internal/app` decides what to show and does the work (tested on Linux); `internal/ui` is Wails glue (compiled with `GOOS=windows`, checked by hand); `internal/ui/web` holds plain HTML/CSS/JS pages. New data comes from small additions to `internal/local` (settings fields, backup reasons and listing), `internal/engine` (who wrote the base snapshot) and `internal/syncdir` (list the PCs in a folder). The page asks Go for one `MainPage` and re-reads it whenever Go emits a `changed` event. The pages have a demo mode (sample data, no Wails) that a script screenshots with headless Chrome for the README.

**Tech Stack:** Go 1.26, Wails `v3.0.0-beta.32` (Windows-only files), plain ES modules (no npm, no build step), headless Chrome for screenshots, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-22-main-window-design.md` (builds on `docs/superpowers/specs/2026-10-19-m2-tray-installer-design.md`).

## Global Constraints

- Layout is **flavor first**: flavor list on the left, the selected flavor's page on the right, **Pull all** / **Push all** / **⚙** in the top bar.
- Everything lives in the main window except the **conflict dialog**, which stays its own window (one per question, closing it = Cancel).
- Tray left-click opens the window; closing it hides it; at login only the tray starts unless the PC isn't set up (then the window opens on the wizard).
- Badges, most important first: `!` red (last action failed, push pending, cloud folder unreachable) > `↓` amber (another PC has newer settings) > `↑ N` amber (N local changes) > `new` grey (never synced) > `✓` green. Missing addons don't change the badge.
- Times are local: `today HH:MM`, `yesterday HH:MM`, `Mon D, HH:MM`, or `Mon D YYYY, HH:MM` in another year. Snapshot file names never appear in the window.
- Backups: newest first; any kept set can be restored; reasons read `before pull from <PC>` / `before restore`; older backups without a recorded reason show only date and file count.
- Appearance: `system` (default), `light`, `dark`; per PC in `settings.json`; applied and saved immediately; unknown values read as `system`.
- All flavors: on by default; on = every flavor, including ones installed later; off = per-flavor switches and a flavor installed later starts off. Settings written before `allFlavors` existed read as all when no listed flavor is off, otherwise as individual choices with unlisted flavors off.
- Tray menu (set up): **Open wtfsync** · Push · Pull · Settings… · Update to vX *(when available)* · Open log folder · Quit. Not set up: **Set up wtfsync…** · Open log folder · Quit.
- Wails stays pinned at `v3.0.0-beta.32`; pages stay plain HTML/CSS/JS embedded with `go:embed`, no build step.
- Lists sent to the pages are never `null` (Wails sends nil slices as `null`).
- Commits use Conventional Commits; never commit to `main`.
- Out of scope: addon sync, changing how many backups/snapshots are kept, file diffs, leaving Wails.

## Review Focus

1. **Reload storm:** the page re-reads `Main` on every `changed` event; if `Main` itself triggered `Changed` (e.g. by calling `Refresh`), the window would loop forever. Expected: `Main` never calls `Changed`. Pinned by `TestMainDoesNotTriggerChanged` (Task 4).
2. **Upgrading with flavors switched off:** an existing `settings.json` with one flavor off and no `allFlavors` must not start syncing that flavor. Pinned by `TestAllAndEnabled` (Task 1).
3. **Snapshot times in UTC shown in local time across midnight:** a 03:51 UTC snapshot is "yesterday 22:51" in UTC−5. Pinned by `TestWhen` (Task 4).
4. **Restoring a backup that was pruned meanwhile** (a pull made an 11th set while the window showed the old list): expected "That backup no longer exists", nothing written. Pinned by `TestRestorePrunedBackupSaysSo` (Task 5).
5. **Per-flavor action while another action runs:** expected `ErrBusy`, nothing runs. Pinned by `TestFlavorActionWhileBusyIsRefused` (Task 5).

---

### Task 1: Settings — All flavors and appearance

**Files:**
- Modify: `internal/local/store.go`
- Test: `internal/local/store_test.go`
- Modify: `internal/setup/setup.go`, `internal/setup/setup_test.go`
- Modify: `internal/cli/sync.go`, `internal/cli/sync_test.go`
- Modify: `internal/app/app.go` (`enabled`, `SettingsInput`, `SaveSettings`), `internal/app/app_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `local.Settings.AllFlavors *bool` (json `allFlavors,omitempty`), `local.Settings.Appearance string` (json `appearance,omitempty`)
  - `func (s local.Settings) All() bool`, `func (s local.Settings) Enabled(flavor string) bool`, `func (s local.Settings) Theme() string`
  - `app.SettingsInput.AllFlavors bool`

- [ ] **Step 1: Write the failing tests**

Append to `internal/local/store_test.go`:

```go
func TestAllAndEnabled(t *testing.T) {
	on, off := true, false
	for _, tt := range []struct {
		name    string
		s       Settings
		all     bool
		enabled map[string]bool // flavor → want
	}{
		{"all on", Settings{AllFlavors: &on, Flavors: map[string]bool{"retail": false}}, true,
			map[string]bool{"retail": true, "classic": true}},
		{"all off picks", Settings{AllFlavors: &off, Flavors: map[string]bool{"retail": true, "classic": false}}, false,
			map[string]bool{"retail": true, "classic": false, "classic_era": false}},
		{"old file, nothing off", Settings{Flavors: map[string]bool{"retail": true}}, true,
			map[string]bool{"retail": true, "classic": true}},
		{"old file, one off", Settings{Flavors: map[string]bool{"retail": true, "classic": false}}, false,
			map[string]bool{"retail": true, "classic": false, "classic_era": false}},
	} {
		if got := tt.s.All(); got != tt.all {
			t.Errorf("%s: All() = %v, want %v", tt.name, got, tt.all)
		}
		for f, want := range tt.enabled {
			if got := tt.s.Enabled(f); got != want {
				t.Errorf("%s: Enabled(%s) = %v, want %v", tt.name, f, got, want)
			}
		}
	}
}

func TestThemeDefaultsToSystem(t *testing.T) {
	for in, want := range map[string]string{"": "system", "system": "system", "light": "light", "dark": "dark", "purple": "system"} {
		if got := (Settings{Appearance: in}).Theme(); got != want {
			t.Errorf("Theme(%q) = %q, want %q", in, got, want)
		}
	}
}
```

Append to `internal/setup/setup_test.go`:

```go
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
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/local/ ./internal/setup/`
Expected: build failure — `unknown field AllFlavors in struct literal`, `tt.s.All undefined`.

- [ ] **Step 3: Implement the settings fields**

In `internal/local/store.go`, replace the `Settings` struct with:

```go
type Settings struct {
	MachineUUID string          `json:"machineUUID"`
	MachineName string          `json:"machineName"` // display label only; never identity
	SyncFolder  string          `json:"syncFolder"`
	WoWDir      string          `json:"wowDir"`
	Flavors     map[string]bool `json:"flavors"` // flavor → on; what syncs when AllFlavors is off

	// AllFlavors syncs every flavor, including ones installed later. nil in settings written
	// before it existed: see All.
	AllFlavors *bool  `json:"allFlavors,omitempty"`
	Appearance string `json:"appearance,omitempty"` // "system" (or ""), "light", "dark"
}

// All reports whether every flavor syncs. Settings written before AllFlavors existed count as all
// when no listed flavor was switched off, so nobody's choices change on upgrade.
func (s Settings) All() bool {
	if s.AllFlavors != nil {
		return *s.AllFlavors
	}
	for _, on := range s.Flavors {
		if !on {
			return false
		}
	}
	return true
}

// Enabled reports whether flavor syncs. When not all flavors sync, one that isn't listed (installed
// since) is off until switched on.
func (s Settings) Enabled(flavor string) bool {
	return s.All() || s.Flavors[flavor]
}

// Theme is the appearance to use: "system", "light" or "dark". Anything else reads as system.
func (s Settings) Theme() string {
	switch s.Appearance {
	case "light", "dark":
		return s.Appearance
	}
	return "system"
}
```

- [ ] **Step 4: Use them in setup, the CLI and the app**

In `internal/setup/setup.go`, replace:

```go
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
```

with:

```go
	all := true
	s := local.Settings{MachineUUID: local.NewUUID(), MachineName: a.Name, SyncFolder: a.SyncFolder, WoWDir: a.WoWDir, Flavors: map[string]bool{}, AllFlavors: &all}
	if res.Rerun {
		s.MachineUUID = prev.MachineUUID
		maps.Copy(s.Flavors, prev.Flavors)
		s.AllFlavors, s.Appearance = prev.AllFlavors, prev.Appearance
	}
	// Listing every flavor found gives the Settings page's switches a starting point. When the
	// user picks flavors individually, one found later stays off (unlisted).
	if s.All() {
		for _, f := range flavors {
			if _, ok := s.Flavors[f.Name]; !ok {
				s.Flavors[f.Name] = true
			}
		}
	}
```

and in the same file replace `if !s.Flavors[f.Name] {` with `if !s.Enabled(f.Name) {`.

In `internal/cli/sync.go`, replace the comment and condition in `selectFlavors`:

```go
// selectFlavors returns the installed flavors to act on: just name if given, else every enabled
// flavor (local.Settings.Enabled).
```

and

```go
		if s.Enabled(f.Name) {
			out = append(out, f)
		}
```

(replacing `if enabled, known := s.Flavors[f.Name]; !known || enabled {`).

In `internal/app/app.go`, in `enabled`, replace `if enabled, listed := s.Flavors[f.Name]; enabled || !listed {` with `if s.Enabled(f.Name) {`.

In `internal/app/app.go`, replace `SettingsInput` and the start of `SaveSettings`:

```go
// SettingsInput is the settings page's form.
type SettingsInput struct {
	Name         string
	WoWDir       string
	SyncFolder   string
	AllFlavors   bool
	Flavors      map[string]bool // used when AllFlavors is off
	StartAtLogin bool
}

// SaveSettings applies the settings page. Changing folders follows the setup rerun rules; a new
// sync folder that already has wtfsync data is joined (link), otherwise this PC starts it (init).
func (a *App) SaveSettings(in SettingsInput) (setup.Result, error) {
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return setup.Result{}, err
	}
	s.AllFlavors = &in.AllFlavors
	for name, on := range in.Flavors {
		s.Flavors[name] = on
	}
	if err := a.d.Store.SaveSettings(s); err != nil {
		return setup.Result{}, err
	}
```

(the rest of `SaveSettings` is unchanged).

- [ ] **Step 5: Update the tests that switch a flavor off**

Picking flavors now means turning All flavors off. In `internal/setup/setup_test.go` `TestRerunKeepsDisabledFlavorsOff`, replace `s.Flavors["retail"] = false` with:

```go
	off := false
	s.AllFlavors = &off
	s.Flavors["retail"] = false
```

In `internal/cli/sync_test.go` `TestNewFlavorIncludedDisabledFlavorSkipped`, replace `s.Flavors["classic"] = false` with:

```go
	off := false
	s.AllFlavors = &off
	s.Flavors["classic"] = false
```

In `internal/app/app_test.go`:
- `TestSaveSettingsRenamesTogglesAndStartsAtLogin`: leave both calls as they are (no `AllFlavors` means off, and `retail` is off).
- `TestSaveSettingsNewEmptyFolderStartsItNewFolderWithDataJoinsIt`: add `AllFlavors: true,` to both `SettingsInput{...}` literals.

Append to `internal/app/app_test.go`:

```go
func TestSaveSettingsAllFlavorsOffKeepsALaterFlavorOff(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	if _, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: ny.sync, Flavors: map[string]bool{"retail": true}}); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(ny.wowDir, "_classic_", "WTF", "Account"), 0o755) // installed later
	nyApp.Refresh()
	if v := nyApp.View(); len(v.Flavors) != 1 || v.Flavors[0].Name != "retail" {
		t.Fatalf("flavors %+v, want only retail", v.Flavors)
	}
	if _, err := nyApp.SaveSettings(SettingsInput{Name: "NY", WoWDir: ny.wowDir, SyncFolder: ny.sync, AllFlavors: true}); err != nil {
		t.Fatal(err)
	}
	nyApp.Refresh()
	if v := nyApp.View(); len(v.Flavors) != 2 {
		t.Fatalf("flavors %+v, want classic and retail", v.Flavors)
	}
}
```

(add `"os"` and `"path/filepath"` to the test file's imports if `go vet` reports them missing.)

- [ ] **Step 6: Run the tests**

Run: `go test ./... && GOOS=windows go vet ./...`
Expected: all packages `ok`; vet prints nothing. (`internal/ui`'s `SaveSettings` still compiles: it leaves `AllFlavors` false until Task 7.)

- [ ] **Step 7: Commit**

```bash
git add internal/local internal/setup internal/cli internal/app
git commit -m "feat: add an All flavors setting and a per-PC appearance setting"
```

---

### Task 2: Backups record why they were made

**Files:**
- Modify: `internal/local/backup.go`, `internal/local/backup_test.go`
- Modify: `internal/engine/engine.go:172`, `internal/engine/status.go:86`
- Test: `internal/engine/status_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `local.ErrNoBackup error`
  - `type local.Why struct { Reason string; From string }` (json `reason`, `from`, both omitempty); `local.ReasonPull = "pull"`, `local.ReasonRestore = "restore"`
  - `func (s local.Store) Backup(flavor, wtfDir string, paths []string, now time.Time, why local.Why) (string, error)`
  - `type local.BackupInfo struct { ID string; Time time.Time; Why; Files int }`
  - `func (s local.Store) Backups(flavor string) ([]local.BackupInfo, error)` — newest first

- [ ] **Step 1: Write the failing tests**

In `internal/local/backup_test.go`, change every `s.Backup(...)` call to pass `Why{}` as a last argument (three calls: `TestBackupAndRestore`, `TestBackupSetsNewestFirstAndPruned`). Then append:

```go
func TestBackupsListNewestFirstWithWhy(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	put(t, wtf, svA, "a")
	put(t, wtf, svB, "b")
	if _, err := s.Backup("retail", wtf, []string{svA}, t0, Why{Reason: ReasonPull, From: "leviathan"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Backup("retail", wtf, []string{svA, svB}, t0.Add(time.Hour), Why{Reason: ReasonRestore}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Backups("retail")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].Reason != ReasonRestore || got[0].Files != 2 || !got[0].Time.Equal(t0.Add(time.Hour)) ||
		got[1].Reason != ReasonPull || got[1].From != "leviathan" || got[1].Files != 1 || !got[1].Time.Equal(t0) {
		t.Fatalf("backups %+v", got)
	}
	if none, err := s.Backups("classic"); err != nil || len(none) != 0 {
		t.Fatalf("no backups: %v, %v", none, err)
	}
}

func TestOldBackupWithoutWhyStillLists(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	put(t, wtf, svA, "a")
	id, err := s.Backup("retail", wtf, []string{svA}, t0, Why{})
	if err != nil {
		t.Fatal(err)
	}
	// What a backup written before v0.8 looks like.
	os.WriteFile(filepath.Join(s.Dir, "backups", "retail", id, "index.json"), []byte(`{"present":["`+svA+`"],"absent":null}`), 0o644)
	got, err := s.Backups("retail")
	if err != nil || len(got) != 1 || got[0].Reason != "" || got[0].Files != 1 {
		t.Fatalf("backups %+v, %v", got, err)
	}
}

func TestMissingBackupIsErrNoBackup(t *testing.T) {
	s, wtf := Store{Dir: t.TempDir()}, t.TempDir()
	if err := s.Restore("retail", "20990101T000000.000Z", wtf); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("err = %v, want ErrNoBackup", err)
	}
}
```

Add `"errors"` to that file's imports.

Append to `internal/engine/status_test.go`:

```go
func TestBackupsSayWhyTheyWereMade(t *testing.T) {
	v := newEnv(t)
	s1 := v.remote(1, map[string]string{pathA: "a1"})
	v.synced(s1, map[string]string{pathA: "a1"})
	v.writeLocal(pathA, "a1")
	v.remote(2, map[string]string{pathA: "a2"})
	if _, err := v.eng.Pull(v.flavor, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.eng.Restore(v.flavor, "", Options{}); err != nil {
		t.Fatal(err)
	}
	got, err := v.eng.Store.Backups("retail")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Reason != local.ReasonRestore || got[1].Reason != local.ReasonPull || got[1].From != "NJ" {
		t.Fatalf("backups %+v", got)
	}
}
```

Add `"github.com/rhwendt/wtfsync/internal/local"` to that file's imports.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/local/ ./internal/engine/`
Expected: build failure — `undefined: Why`, `s.Backups undefined`, `undefined: ErrNoBackup`.

- [ ] **Step 3: Implement**

In `internal/local/backup.go`:

Replace `type backupIndex struct {...}` with:

```go
// ErrNoBackup means the backup set doesn't exist: never made, or pruned since.
var ErrNoBackup = errors.New("no such backup")

const (
	ReasonPull    = "pull"    // made before a pull (or a push's merge) overwrote files
	ReasonRestore = "restore" // made before a restore overwrote files
)

// Why records what a backup set was made before. Both are empty in backups made before v0.8.
type Why struct {
	Reason string `json:"reason,omitempty"` // ReasonPull or ReasonRestore
	From   string `json:"from,omitempty"`   // the PC pulled from (ReasonPull)
}

type backupIndex struct {
	Present []string `json:"present"` // backed-up files, under files/
	Absent  []string `json:"absent"`  // paths that didn't exist locally at backup time
	Why
}

// BackupInfo describes one backup set.
type BackupInfo struct {
	ID   string
	Time time.Time // when it was made, UTC
	Why
	Files int // files a restore rewrites or deletes
}
```

Change `Backup`'s signature and index to:

```go
func (s Store) Backup(flavor, wtfDir string, paths []string, now time.Time, why Why) (string, error) {
	id := now.UTC().Format(backupLayout)
	dir := s.backupDir(flavor, id)
	idx := backupIndex{Why: why}
```

(the rest of `Backup` is unchanged).

Add after `BackupSets`:

```go
// Backups lists a flavor's backup sets, newest first.
func (s Store) Backups(flavor string) ([]BackupInfo, error) {
	ids, err := s.BackupSets(flavor)
	if err != nil {
		return nil, err
	}
	out := []BackupInfo{}
	for _, id := range ids {
		idx, err := s.readIndex(flavor, id)
		if err != nil {
			return nil, err
		}
		t, _ := time.Parse(backupLayout, id)
		out = append(out, BackupInfo{ID: id, Time: t, Why: idx.Why, Files: len(idx.Present) + len(idx.Absent)})
	}
	return out, nil
}
```

In `readIndex`, replace `return idx, fmt.Errorf("no backup %q for %s", id, flavor)` with:

```go
			return idx, fmt.Errorf("%w: %q for %s", ErrNoBackup, id, flavor)
```

In `internal/engine/engine.go`, change the pull's backup call to:

```go
		if rep.Backup, err = e.Store.Backup(f.Name, f.WTFDir(), take, e.Now(), local.Why{Reason: local.ReasonPull, From: m.Machine.Name}); err != nil {
```

In `internal/engine/status.go`, change the restore's undo backup call to:

```go
	if rep.Undo, err = e.Store.Backup(f.Name, f.WTFDir(), paths, e.Now(), local.Why{Reason: local.ReasonRestore}); err != nil {
```

(add the `local` import to either file if it's missing.)

- [ ] **Step 4: Run the tests**

Run: `go test ./... && GOOS=windows go vet ./...`
Expected: all `ok`, vet silent.

- [ ] **Step 5: Commit**

```bash
git add internal/local internal/engine
git commit -m "feat: record why each backup was made and list backups with it"
```

---

### Task 3: Who wrote the base snapshot; the PCs in a sync folder

**Files:**
- Modify: `internal/engine/status.go`, `internal/engine/status_test.go`
- Modify: `internal/syncdir/dir.go`, `internal/syncdir/dir_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `engine.Status.BaseBy string` — display name of the PC that wrote `Base`; `""` when unknown
  - `func (d *syncdir.Dir) Machines() ([]syncdir.MachineRecord, error)` — sorted by name

- [ ] **Step 1: Write the failing tests**

In `internal/engine/status_test.go` `TestStatus`, change `want` to include `BaseBy: "NJ"`:

```go
	want := Status{Flavor: "retail", Base: s1.String(), BaseBy: "NJ", Newest: s2.String(), NewestBy: "NJ", Reachable: true, LocalChanges: 2}
```

Append to `internal/syncdir/dir_test.go`:

```go
func TestMachinesListsEveryPCByName(t *testing.T) {
	d, err := Init(t.TempDir(), now, h)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := d.Machines(); err != nil || len(got) != 0 {
		t.Fatalf("empty folder: %v, %v", got, err)
	}
	d.WriteMachine(MachineRecord{UUID: "00000000-0000-4000-8000-000000000002", Name: "leviathan", LastSeen: now})
	d.WriteMachine(MachineRecord{UUID: "00000000-0000-4000-8000-000000000001", Name: "beehive", LastSeen: now})
	os.WriteFile(filepath.Join(d.Root, "machines", "notes.txt"), []byte("x"), 0o644) // not a record
	got, err := d.Machines()
	if err != nil || len(got) != 2 || got[0].Name != "beehive" || got[1].Name != "leviathan" {
		t.Fatalf("machines %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/engine/ ./internal/syncdir/`
Expected: build failure — `unknown field BaseBy`, `d.Machines undefined`.

- [ ] **Step 3: Implement**

In `internal/engine/status.go`, add the field after `Base`:

```go
	BaseBy       string // display name of the PC that wrote Base; "" when unknown
```

and in `Status`, right after `s.MissingAddons = e.missingAddons(f, d)`:

```go
	if b, ok := syncdir.ParseName(s.Base); ok {
		if m, err := d.Machine(b.UUID); err == nil {
			s.BaseBy = m.Name
		}
	}
```

In `internal/syncdir/dir.go`, add after `Machine`:

```go
// Machines lists every PC that has joined this folder, sorted by name. Files that aren't readable
// records are skipped.
func (d *Dir) Machines() ([]MachineRecord, error) {
	entries, err := os.ReadDir(filepath.Join(d.Root, "machines"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []MachineRecord
	for _, e := range entries {
		uuid, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() {
			continue
		}
		if r, err := d.Machine(uuid); err == nil {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b MachineRecord) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}
```

(add `"io/fs"`, `"slices"`, `"strings"` to `dir.go`'s imports as needed.)

- [ ] **Step 4: Run the tests**

Run: `go test ./... && GOOS=windows go vet ./...`
Expected: all `ok`, vet silent.

- [ ] **Step 5: Commit**

```bash
git add internal/engine internal/syncdir
git commit -m "feat: know which PC wrote the base snapshot and list a folder's PCs"
```

---

### Task 4: The main window's data

**Files:**
- Create: `internal/app/page.go`
- Test: `internal/app/page_test.go`

**Interfaces:**
- Consumes: `engine.Status.BaseBy` (Task 3); `local.Store.Backups`, `local.BackupInfo`, `local.ReasonPull`, `local.ReasonRestore` (Task 2); `local.Settings.Theme` (Task 1).
- Produces (all in package `app`):
  - `type Badge struct { Kind, Short, Words string }` — Kind is `"error"`, `"newer"`, `"changes"`, `"new"` or `"ok"`
  - `type BackupItem struct { ID, When, Reason string; Files int }`
  - `type MissingAddon struct { Name, URL, Folders string }`
  - `type MissingGroup struct { PC string; Addons []MissingAddon }`
  - `type FlavorPage struct { Name string; Badge Badge; Line string; Backups []BackupItem; Missing []MissingGroup }`
  - `type MainPage struct { SetUp bool; Busy, PC, Version, Update, Appearance string; Flavors []FlavorPage }`
  - `func (a *App) Main() MainPage` — never calls `Changed` or `Refresh`
  - unexported, tested: `badge(FlavorView) Badge`, `when(t, now time.Time) string`, `statusLine(engine.Status, time.Time) string`

- [ ] **Step 1: Write the failing tests**

Create `internal/app/page_test.go`:

```go
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

func TestBadgeMostImportantFirst(t *testing.T) {
	ok := engine.Status{Reachable: true, Base: "b", Newest: "b"}
	for _, tt := range []struct {
		f    FlavorView
		want Badge
	}{
		{FlavorView{Name: "retail", Err: "Close WoW (retail) first", Status: engine.Status{PendingPush: true}}, Badge{"error", "!", "Close WoW (retail) first"}},
		{FlavorView{Status: engine.Status{PendingPush: true, Reachable: true}}, Badge{"error", "!", "Push pending: cloud folder unreachable"}},
		{FlavorView{Status: engine.Status{Base: "b"}}, Badge{"error", "!", "Cloud folder unreachable"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "a", Newest: "b", NewestBy: "leviathan", LocalChanges: 3}}, Badge{"newer", "↓", "leviathan has newer settings"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "b", Newest: "b", LocalChanges: 26}}, Badge{"changes", "↑ 26", "26 changes to push"}},
		{FlavorView{Status: engine.Status{Reachable: true, Base: "b", Newest: "b", LocalChanges: 1}}, Badge{"changes", "↑ 1", "1 change to push"}},
		{FlavorView{Status: engine.Status{Reachable: true}}, Badge{"new", "new", "Not synced yet"}},
		{FlavorView{Status: ok}, Badge{"ok", "✓", "In sync"}},
	} {
		if got := badge(tt.f); got != tt.want {
			t.Errorf("badge(%+v) = %+v, want %+v", tt.f, got, tt.want)
		}
	}
}

func TestWhen(t *testing.T) {
	est := time.FixedZone("EST", -5*3600)
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, est)
	for _, tt := range []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 10, 22, 8, 0, 0, 0, est), "today 08:00"},
		{time.Date(2026, 10, 22, 3, 51, 0, 0, time.UTC), "yesterday 22:51"}, // UTC morning is the evening before here
		{time.Date(2026, 10, 19, 2, 7, 0, 0, time.UTC), "Oct 18, 21:07"},
		{time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC), "Dec 31 2025, 07:00"},
	} {
		if got := when(tt.t, now); got != tt.want {
			t.Errorf("when(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestStatusLine(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	name := func(at time.Time) string { return syncdir.SnapshotName{Seq: 1, Time: at, UUID: "u"}.String() }
	today := name(time.Date(2026, 10, 22, 3, 51, 0, 0, time.UTC))
	oct19 := name(time.Date(2026, 10, 19, 2, 7, 0, 0, time.UTC))
	oct20 := name(time.Date(2026, 10, 20, 3, 51, 0, 0, time.UTC))
	for _, tt := range []struct {
		s    engine.Status
		want string
	}{
		{engine.Status{}, "Never synced"},
		{engine.Status{Newest: oct20, NewestBy: "leviathan"}, "Never synced · leviathan pushed Oct 20, 03:51"},
		{engine.Status{Base: today, BaseBy: "leviathan", Newest: today, NewestBy: "leviathan"}, "Last synced today 03:51 · newest from leviathan"},
		{engine.Status{Base: today, Newest: today}, "Last synced today 03:51"},
		{engine.Status{Base: oct19, BaseBy: "beehive", Newest: oct20, NewestBy: "leviathan"}, "Last synced Oct 19, 02:07 · leviathan pushed Oct 20, 03:51"},
		{engine.Status{Base: oct19}, "Last synced Oct 19, 02:07"}, // cloud folder unreachable: no newest
	} {
		if got := statusLine(tt.s, now); got != tt.want {
			t.Errorf("statusLine(%+v) = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestMainShowsBackupsAndMissingAddons(t *testing.T) {
	nj, _, njApp, nyApp := linked(t)
	nj.install("Plater", "## X-Curse-Project-ID: 100547")
	nj.write(svA, "a-nj")
	njApp.Push()
	if err := nyApp.Pull(); err != nil {
		t.Fatal(err)
	}
	m := nyApp.Main()
	if !m.SetUp || m.PC != "NY" || m.Version != "v0.5.0" || m.Appearance != "system" || len(m.Flavors) != 1 {
		t.Fatalf("main %+v", m)
	}
	f := m.Flavors[0]
	if f.Name != "retail" || f.Badge.Kind != "ok" {
		t.Fatalf("flavor %+v", f)
	}
	if !strings.HasPrefix(f.Line, "Last synced today ") || !strings.HasSuffix(f.Line, " · newest from NJ") {
		t.Fatalf("line %q", f.Line)
	}
	// Newest first: this pull's backup (linking made an earlier one).
	if len(f.Backups) == 0 || f.Backups[0].Reason != "before pull from NJ" || f.Backups[0].Files != 1 || !strings.HasPrefix(f.Backups[0].When, "today ") {
		t.Fatalf("backups %+v", f.Backups)
	}
	want := []MissingGroup{{PC: "NJ", Addons: []MissingAddon{{Name: "Plater", URL: "https://www.curseforge.com/projects/100547", Folders: "Plater"}}}}
	if !reflect.DeepEqual(f.Missing, want) {
		t.Fatalf("missing %+v", f.Missing)
	}
}

func TestMainDoesNotTriggerChanged(t *testing.T) {
	_, _, _, nyApp := linked(t)
	changed := 0
	nyApp.d.Changed = func() { changed++ }
	nyApp.Main()
	if changed != 0 {
		t.Fatalf("Main called Changed %d time(s): the window would reload forever", changed)
	}
}

func TestMainListsAreNeverNull(t *testing.T) {
	_, ny := newPair(t)
	b, err := json.Marshal(ny.app().Main()) // not set up: no flavors at all
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("json has null: %s", b)
	}
	_, _, _, nyApp := linked(t)
	if b, _ := json.Marshal(nyApp.Main()); strings.Contains(string(b), "null") {
		t.Fatalf("json has null: %s", b)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/app/`
Expected: build failure — `undefined: Badge`, `undefined: badge`, `undefined: when`, `nyApp.Main undefined`.

- [ ] **Step 3: Implement**

Create `internal/app/page.go`:

```go
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/rhwendt/wtfsync/internal/engine"
	"github.com/rhwendt/wtfsync/internal/local"
	"github.com/rhwendt/wtfsync/internal/syncdir"
)

// MainPage is everything the main window shows.
type MainPage struct {
	SetUp      bool
	Busy       string // what's running ("pulling classic"); "" when idle
	PC         string // this PC's name
	Version    string
	Update     string // newer version on offer; "" when none
	Appearance string // "system", "light" or "dark"
	Flavors    []FlavorPage
}

// FlavorPage is one flavor: its row in the list and its page.
type FlavorPage struct {
	Name    string
	Badge   Badge
	Line    string // "Last synced today 03:51 · newest from leviathan"
	Backups []BackupItem
	Missing []MissingGroup
}

// Badge is a flavor's state in a few characters (the list) and in words (its page).
type Badge struct {
	Kind  string // "error", "newer", "changes", "new" or "ok": the colour
	Short string // "!", "↓", "↑ 26", "new", "✓"
	Words string // "Cloud folder unreachable", "leviathan has newer settings", "26 changes to push", …
}

// BackupItem is one backup set in the list.
type BackupItem struct {
	ID     string
	When   string // "today 03:51"
	Reason string // "before pull from leviathan", "before restore", or "" for older backups
	Files  int
}

// MissingGroup is the addons one other PC has that this one doesn't.
type MissingGroup struct {
	PC     string
	Addons []MissingAddon
}

type MissingAddon struct {
	Name    string
	URL     string // download page; "" when unknown
	Folders string // "ElvUI, ElvUI_Options"
}

// Main is the main window's data. It reads what the last refresh found plus the backups on disk;
// it never refreshes or calls Changed, because the window reloads it on every Changed.
func (a *App) Main() MainPage {
	v := a.View()
	m := MainPage{SetUp: v.SetUp, Busy: v.Busy, Version: a.d.Version, Update: v.Update, Appearance: "system", Flavors: []FlavorPage{}}
	if s, ok, _ := a.Settings(); ok {
		m.PC, m.Appearance = s.MachineName, s.Theme()
	}
	now := a.d.Now()
	for _, f := range v.Flavors {
		m.Flavors = append(m.Flavors, FlavorPage{
			Name:    f.Name,
			Badge:   badge(f),
			Line:    statusLine(f.Status, now),
			Backups: a.backups(f.Name, now),
			Missing: missingGroups(f.Status),
		})
	}
	return m
}

func (a *App) backups(flavor string, now time.Time) []BackupItem {
	out := []BackupItem{}
	sets, err := a.d.Store.Backups(flavor)
	if err != nil {
		a.d.Log.Printf("backups %s: %v", flavor, err)
		return out
	}
	for _, b := range sets {
		item := BackupItem{ID: b.ID, When: when(b.Time, now), Files: b.Files}
		switch {
		case b.Reason == local.ReasonPull && b.From != "":
			item.Reason = "before pull from " + b.From
		case b.Reason == local.ReasonPull:
			item.Reason = "before pull"
		case b.Reason == local.ReasonRestore:
			item.Reason = "before restore"
		}
		out = append(out, item)
	}
	return out
}

func missingGroups(s engine.Status) []MissingGroup {
	out := []MissingGroup{}
	for _, o := range s.MissingAddons {
		g := MissingGroup{PC: o.Machine, Addons: []MissingAddon{}}
		for _, m := range o.Addons {
			g.Addons = append(g.Addons, MissingAddon{Name: m.Name, URL: m.URL, Folders: strings.Join(m.Folders, ", ")})
		}
		out = append(out, g)
	}
	return out
}

// badge is the flavor's most important state.
func badge(f FlavorView) Badge {
	s := f.Status
	switch {
	case f.Err != "":
		return Badge{"error", "!", f.Err}
	case s.PendingPush:
		return Badge{"error", "!", "Push pending: cloud folder unreachable"}
	case !s.Reachable:
		return Badge{"error", "!", "Cloud folder unreachable"}
	case f.newer():
		return Badge{"newer", "↓", s.NewestBy + " has newer settings"}
	case s.LocalChanges > 0:
		return Badge{"changes", fmt.Sprintf("↑ %d", s.LocalChanges), plural(s.LocalChanges, "change") + " to push"}
	case s.Base == "":
		return Badge{"new", "new", "Not synced yet"}
	}
	return Badge{"ok", "✓", "In sync"}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// statusLine says when the flavor last synced and what's newest, in local time.
func statusLine(s engine.Status, now time.Time) string {
	base, hasBase := syncdir.ParseName(s.Base)
	newest, hasNewest := syncdir.ParseName(s.Newest)
	switch {
	case !hasBase && hasNewest:
		return fmt.Sprintf("Never synced · %s pushed %s", s.NewestBy, when(newest.Time, now))
	case !hasBase:
		return "Never synced"
	case !hasNewest:
		return "Last synced " + when(base.Time, now)
	case s.Base == s.Newest && s.BaseBy != "":
		return fmt.Sprintf("Last synced %s · newest from %s", when(base.Time, now), s.BaseBy)
	case s.Base == s.Newest:
		return "Last synced " + when(base.Time, now)
	}
	return fmt.Sprintf("Last synced %s · %s pushed %s", when(base.Time, now), s.NewestBy, when(newest.Time, now))
}

// when is t in now's time zone, as a person would say it.
func when(t, now time.Time) string {
	t = t.In(now.Location())
	sameDay := func(a, b time.Time) bool {
		ay, am, ad := a.Date()
		by, bm, bd := b.Date()
		return ay == by && am == bm && ad == bd
	}
	switch {
	case sameDay(t, now):
		return "today " + t.Format("15:04")
	case sameDay(t, now.AddDate(0, 0, -1)):
		return "yesterday " + t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("Jan 2, 15:04")
	}
	return t.Format("Jan 2 2006, 15:04")
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/app/ && go test ./... && GOOS=windows go vet ./...`
Expected: all `ok`, vet silent.

- [ ] **Step 5: Commit**

```bash
git add internal/app/page.go internal/app/page_test.go
git commit -m "feat: describe each flavor for the main window: badge, times, backups, missing addons"
```

---

### Task 5: App actions for the window, and the new tray menu

**Files:**
- Modify: `internal/app/app.go`, `internal/app/view.go`
- Test: `internal/app/app_test.go`, `internal/app/view_test.go`, `internal/app/diag_test.go`

**Interfaces:**
- Consumes: `local.ErrNoBackup` (Task 2); `syncdir.Dir.Machines` (Task 3); `local.Settings.Theme` (Task 1).
- Produces:
  - `func (a *App) PushFlavor(name string) error`, `func (a *App) PullFlavor(name string) error` — busy text `"pushing <name>"` / `"pulling <name>"`
  - `func (a *App) RestorePreview(flavor, id string) (engine.RestoreReport, error)`, `func (a *App) Restore(flavor, id string) (engine.RestoreReport, error)` — `id ""` = newest; a missing set returns an error wrapping `local.ErrNoBackup` and the flavor's error reads "That backup no longer exists"
  - `func (a *App) SetAppearance(v string) error` — `"system"`, `"light"`, `"dark"`; calls `Changed`
  - `func (a *App) CloudPCs(folder string) []string` — names of PCs in that folder's wtfsync data, except this PC; empty when none
  - `func (a *App) CheckUpdate(ctx context.Context) error` (now returns the error it logs)
  - `Menu(v)` set up: `open, push, pull, settings, [update], -, logs, quit`

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/app_test.go`:

```go
func TestFlavorActionsActOnOneFlavor(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a-nj")
	if err := njApp.PushFlavor("retail"); err != nil {
		t.Fatal(err)
	}
	if err := nyApp.PullFlavor("retail"); err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a-nj")
	if err := nyApp.PullFlavor("classic"); err == nil {
		t.Fatal("pulling a flavor that isn't enabled should fail")
	}
}

func TestFlavorActionWhileBusyIsRefused(t *testing.T) {
	_, _, _, nyApp := linked(t)
	nyApp.begin("pulling")
	defer nyApp.end()
	if err := nyApp.PushFlavor("retail"); !errors.Is(err, ErrBusy) {
		t.Fatalf("push: %v", err)
	}
	if _, err := nyApp.Restore("retail", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("restore: %v", err)
	}
}

func TestRestoreChosenBackup(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a1")
	njApp.Push()
	nyApp.Pull() // backup 1 holds a0
	nj.write(svA, "a2")
	njApp.Push()
	nyApp.Pull() // backup 2 holds a1
	sets := nyApp.Main().Flavors[0].Backups // newest first; linking made one more at the end
	if len(sets) < 2 {
		t.Fatalf("backups %+v", sets)
	}
	older := sets[1].ID
	prev, err := nyApp.RestorePreview("retail", older)
	if err != nil || !reflect.DeepEqual(prev.Paths, []string{svA}) {
		t.Fatalf("preview %+v, %v", prev, err)
	}
	rep, err := nyApp.Restore("retail", older)
	if err != nil {
		t.Fatal(err)
	}
	ny.want(svA, "a0")
	after := nyApp.Main().Flavors[0].Backups
	if after[0].ID != rep.Undo || after[0].Reason != "before restore" {
		t.Fatalf("undo set %q not first: %+v", rep.Undo, after)
	}
}

func TestRestorePrunedBackupSaysSo(t *testing.T) {
	nj, ny, njApp, nyApp := linked(t)
	nj.write(svA, "a1")
	njApp.Push()
	nyApp.Pull()
	_, err := nyApp.Restore("retail", "20000101T000000.000Z")
	if !errors.Is(err, local.ErrNoBackup) {
		t.Fatalf("err = %v, want ErrNoBackup", err)
	}
	ny.want(svA, "a1") // nothing written
	if b := nyApp.Main().Flavors[0].Badge; b.Words != "That backup no longer exists" {
		t.Fatalf("badge %+v", b)
	}
}

func TestSetAppearance(t *testing.T) {
	_, _, _, nyApp := linked(t)
	changed := 0
	nyApp.d.Changed = func() { changed++ }
	if err := nyApp.SetAppearance("dark"); err != nil {
		t.Fatal(err)
	}
	if nyApp.Main().Appearance != "dark" || changed == 0 {
		t.Fatalf("appearance %q, changed %d", nyApp.Main().Appearance, changed)
	}
	if err := nyApp.SetAppearance("purple"); err == nil {
		t.Fatal("unknown appearance accepted")
	}
	_, fresh := newPair(t)
	if err := fresh.app().SetAppearance("dark"); err == nil {
		t.Fatal("appearance saved on a PC that isn't set up")
	}
}

func TestCloudPCsNamesTheOtherPCs(t *testing.T) {
	nj, _, njApp, nyApp := linked(t)
	if got := nyApp.CloudPCs(nj.sync); !reflect.DeepEqual(got, []string{"NJ"}) {
		t.Fatalf("from NY: %v", got)
	}
	if got := njApp.CloudPCs(nj.sync); !reflect.DeepEqual(got, []string{"NY"}) {
		t.Fatalf("from NJ: %v", got)
	}
	if got := nyApp.CloudPCs(t.TempDir()); got == nil || len(got) != 0 {
		t.Fatalf("empty folder: %#v", got)
	}
}

func TestCheckUpdateReturnsItsError(t *testing.T) {
	_, ny, _, nyApp := linked(t)
	ny.updates.err = errOffline
	if err := nyApp.CheckUpdate(context.Background()); !errors.Is(err, errOffline) {
		t.Fatalf("err = %v", err)
	}
}
```

Add `"github.com/rhwendt/wtfsync/internal/local"` to the test imports.

Update existing tests in `internal/app/app_test.go`:

- `TestRestoreOneFlavor`: `nyApp.RestorePreview("retail")` → `nyApp.RestorePreview("retail", "")`; `nyApp.Restore("retail")` → `nyApp.Restore("retail", "")`.
- `TestNotSetUpOffersSetupOnly`: unchanged (`setup, -, logs, quit`).
- `TestInSyncAfterSetup`: `want := []string{"open", "push", "pull", "settings", "-", "logs", "quit"}`.
- `TestUpdateOfferedOnlyWhenNewer`: the update item is still 4th from the end — `last[len(last)-4]` stays.
- `TestPullAnnouncesMissingAddonsAndMenuListsThem`: rename to `TestPullAnnouncesMissingAddons` and delete the block from `found := false` through its closing `}` of `if !found {...}` (the menu no longer lists missing addons; the window does).

In `internal/app/view_test.go` `TestMenuDisablesSyncWhileBusy`, change the enabled set to:

```go
		want := it.ID == "open" || it.ID == "logs" || it.ID == "quit"
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/app/`
Expected: build failure — `nyApp.PushFlavor undefined`, `too many arguments in call to nyApp.Restore`, `nyApp.SetAppearance undefined`, `nyApp.CloudPCs undefined`, `nyApp.CheckUpdate(...) (no value) used as value`.

- [ ] **Step 3: Implement the actions**

In `internal/app/app.go`, replace `Push` and `Pull` with:

```go
// Push pushes every enabled flavor.
func (a *App) Push() error { return a.push("") }

// PushFlavor pushes one enabled flavor.
func (a *App) PushFlavor(name string) error { return a.push(name) }

func (a *App) push(only string) error {
	return a.run(doing("pushing", only), only, func(eng *engine.Engine, f wow.Flavor) error {
		_, err := eng.Push(f, engine.Options{})
		return err
	})
}

// Pull pulls every enabled flavor (retrying a pending push first, as the CLI does) and then
// announces addons other PCs have that this one doesn't, when that list has grown.
func (a *App) Pull() error { return a.pull("") }

// PullFlavor pulls one enabled flavor.
func (a *App) PullFlavor(name string) error { return a.pull(name) }

func (a *App) pull(only string) error {
	err := a.run(doing("pulling", only), only, func(eng *engine.Engine, f wow.Flavor) error {
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

// doing is the busy text: "pulling", or "pulling classic" for one flavor.
func doing(what, only string) string {
	if only == "" {
		return what
	}
	return what + " " + only
}
```

In `run`, after the flavor loop and before `return errors.Join(errs...)`, make an `only` flavor that isn't enabled an error:

```go
	if only != "" && !slices.ContainsFunc(flavors, func(f wow.Flavor) bool { return f.Name == only }) {
		return fmt.Errorf("no enabled flavor %q", only)
	}
	return errors.Join(errs...)
```

Replace `RestorePreview` and `Restore` with:

```go
// RestorePreview lists what restoring one of flavor's backup sets (id "" = the newest) would change.
func (a *App) RestorePreview(flavor, id string) (engine.RestoreReport, error) {
	eng, f, err := a.flavor(flavor)
	if err != nil {
		return engine.RestoreReport{}, err
	}
	return eng.Restore(f, id, engine.Options{DryRun: true})
}

// Restore puts one of flavor's backup sets back (id "" = the newest), backing up what it replaces.
func (a *App) Restore(flavor, id string) (engine.RestoreReport, error) {
	var rep engine.RestoreReport
	err := a.run("restoring "+flavor, flavor, func(eng *engine.Engine, f wow.Flavor) error {
		var err error
		rep, err = eng.Restore(f, id, engine.Options{})
		return err
	})
	return rep, err
}
```

In `internal/app/view.go` `friendly`, add a case before the default:

```go
	case errors.Is(err, local.ErrNoBackup):
		return "That backup no longer exists"
```

(add the `local` import.)

Change `CheckUpdate` to return its error:

```go
// CheckUpdate looks for a newer release and offers it. Failures are logged and returned but never
// raised as notifications: being offline mustn't nag.
func (a *App) CheckUpdate(ctx context.Context) error {
	if a.d.Updates == nil {
		return nil
	}
	r, err := a.d.Updates.Latest(ctx)
	if err != nil {
		a.d.Log.Printf("check for updates: %v", err)
		return err
	}
	a.mu.Lock()
	if update.Newer(a.d.Version, r.Version) {
		a.update = &r
	} else {
		a.update = nil
	}
	a.mu.Unlock()
	a.d.Changed()
	return nil
}
```

Add after `StartsAtLogin`:

```go
// SetAppearance saves this PC's appearance: "system", "light" or "dark".
func (a *App) SetAppearance(v string) error {
	switch v {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("unknown appearance %q", v)
	}
	s, err := a.d.Store.LoadSettings()
	if err != nil {
		return err
	}
	s.Appearance = v
	if err := a.d.Store.SaveSettings(s); err != nil {
		return err
	}
	a.d.Changed()
	return nil
}

// CloudPCs names the PCs whose wtfsync data is in folder, other than this one; empty when the folder
// has none (or can't be read). The wizard uses it to offer "Join <PC>".
func (a *App) CloudPCs(folder string) []string {
	names := []string{}
	d, err := syncdir.Open(folder, a.d.Hydrator)
	if err != nil {
		return names
	}
	me, _ := a.d.Store.LoadSettings()
	machines, _ := d.Machines()
	for _, m := range machines {
		if m.UUID != me.MachineUUID {
			names = append(names, m.Name)
		}
	}
	return names
}
```

(`slices` and `syncdir` are already imported in `app.go`; add them if `go vet` says otherwise.)

- [ ] **Step 4: Implement the tray menu**

In `internal/app/view.go`, replace the set-up branch of `Menu`:

```go
	idle := v.Busy == ""
	m := []MenuItem{
		{"open", "Open wtfsync", true},
		{"push", "Push", idle},
		{"pull", "Pull", idle},
		{"settings", "Settings…", idle},
	}
	if v.Update != "" {
		m = append(m, MenuItem{"update", "Update to " + v.Update, idle})
	}
	return append(m, MenuItem{"-", "", false}, logs, MenuItem{"quit", "Quit", true})
```

- [ ] **Step 5: Run the tests**

Run: `go test -race ./internal/app/ && go test ./... && GOOS=windows go vet ./...`
Expected: `internal/app` ok with `-race`; all packages ok. `GOOS=windows go vet` **fails** in `internal/ui` (`RestorePreview` and `Restore` now take a backup id) — this is expected and fixed in Task 7. Confirm every vet error is in `internal/ui/bridge_windows.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/app
git commit -m "feat: per-flavor push, pull and restore, appearance, and an Open wtfsync tray item"
```

---

### Task 6: The main window's pages

**Files:**
- Modify: `internal/ui/web/app.css` (rewrite), `internal/ui/web/app.js` (rewrite), `internal/ui/web/conflicts.html` (rewrite)
- Create: `internal/ui/web/main.html`, `internal/ui/web/main.js`, `internal/ui/web/wizard.js`, `internal/ui/web/conflicts.js`, `internal/ui/web/demo.js`
- Delete: `internal/ui/web/status.html`, `restore.html`, `addons.html`, `settings.html`, `wizard.html`
- Create: `.github/scripts/screenshots.sh`

**Interfaces:**
- Consumes (Bridge methods the pages call; Task 7 implements them in Go, `demo.js` fakes them):
  - `Main() MainPage` · `PushAll()` · `PullAll()` · `PushFlavor(name)` · `PullFlavor(name)`
  - `RestorePreview(flavor, id) []string` (paths) · `Restore(flavor, id) string` (the undo set's id)
  - `Settings() SettingsForm` (`{Name, WoWDir, SyncFolder, AllFlavors, Flavors: [{Name, On}], StartAtLogin}`) · `SaveSettings(SettingsForm) []string`
  - `SetAppearance(v)` · `CheckUpdate()` · `InstallUpdate()` · `OpenLogFolder()` · `OpenURL(url)`
  - `Defaults() WizardDefaults` (`{Installs, Clouds, WoWDir, Sync, Name, Rerun}`) · `Browse(title) string` · `CloudPCs(folder) []string` · `Setup(Answers) []string`
  - `Conflicts() ConflictRequest` (`{ID, Other, FirstLink, Files}`) · `AnswerConflicts(id, choices, cancel)`
  - Events from Go: `changed` (no data), `show` (data: `"settings"`, `"wizard"` or `""`)
- Produces: pages `main.html` (views `flavor`, `settings`, `wizard`) and `conflicts.html`; URL parameters `?view=settings|wizard`, `?demo`, `&flavor=<name>`, `&theme=light|dark`, `&step=<1-5>` (demo only); `.github/scripts/screenshots.sh [out-dir]` (honours `$CHROME`).

- [ ] **Step 1: Shared styles**

Replace `internal/ui/web/app.css` with:

```css
/* wtfsync's pages. Light by default; dark when Windows is dark (Appearance: System) or when the
   page sets data-theme="dark". The badge colours match the tray icon. */
:root {
  --bg: #fff; --side: #f6f8fa; --fg: #1f2328; --muted: #6e7781; --line: #d8dee4; --sel: #e7eef8;
  --btn: #f6f8fa; --accent: #2f7fd8; --bar: #1e2530;
  --ok: #2e9e5b; --warn: #e8a317; --bad: #d63b3b; --grey: #8c959f;
  color-scheme: light;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #1f1f1f; --side: #181818; --fg: #e6e6e6; --muted: #9a9a9a; --line: #333; --sel: #26364a;
    --btn: #2b2b2b; --accent: #4a95e8; color-scheme: dark;
  }
}
:root[data-theme="dark"] {
  --bg: #1f1f1f; --side: #181818; --fg: #e6e6e6; --muted: #9a9a9a; --line: #333; --sel: #26364a;
  --btn: #2b2b2b; --accent: #4a95e8; color-scheme: dark;
}

* { box-sizing: border-box; }
html, body { height: 100%; margin: 0; }
body { font: 14px/1.45 "Segoe UI", system-ui, sans-serif; color: var(--fg); background: var(--bg); }
h1 { font-size: 18px; margin: 0; }
h2 { font-size: 12px; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); margin: 20px 0 8px; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
code { font: 12px Consolas, monospace; }
button { font: inherit; font-size: 13px; padding: 4px 12px; border-radius: 6px; border: 1px solid var(--line);
  background: var(--btn); color: var(--fg); cursor: pointer; white-space: nowrap; }
button.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
button:disabled { opacity: .5; cursor: default; }
input[type=text], select { font: inherit; padding: 5px 8px; border: 1px solid var(--line); border-radius: 6px;
  background: var(--bg); color: var(--fg); min-width: 0; flex: 1; }
.muted { color: var(--muted); font-size: 13px; }
.error { color: var(--bad); }
.grow { flex: 1; }
.row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin: 8px 0; }
.row.end { justify-content: flex-end; margin-top: 20px; }

/* Main window frame */
body.app { display: flex; flex-direction: column; }
.top { display: flex; align-items: center; gap: 8px; padding: 8px 12px; background: var(--bar); color: #fff; }
.top .logo { background: #fff; color: var(--bar); border-radius: 4px; padding: 0 5px; font-size: 12px; font-weight: 800; }
.top .title { font-weight: 600; }
.top .busy { color: #c9d1d9; font-size: 13px; margin-left: 8px; }
.top button { background: #2b3442; border-color: #3b4656; color: #fff; }
.body { flex: 1; display: flex; min-height: 0; }
.list { width: 190px; flex: none; background: var(--side); border-right: 1px solid var(--line); overflow-y: auto;
  display: flex; flex-direction: column; }
.list .item { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border: 0; border-bottom: 1px solid var(--line);
  border-radius: 0; background: transparent; text-align: left; font-weight: 600; }
.list .item.on { background: var(--sel); box-shadow: inset 3px 0 0 var(--accent); }
.list .item .name { flex: 1; overflow: hidden; text-overflow: ellipsis; }
.list .me { margin-top: auto; padding: 10px 12px; font-size: 12px; color: var(--muted); }
main { flex: 1; overflow-y: auto; padding: 16px 20px; min-width: 0; }
#note:empty { display: none; }
#note { margin-bottom: 12px; padding: 8px 10px; border-radius: 6px; background: var(--sel); white-space: pre-wrap; }
#note.error { background: transparent; border: 1px solid var(--bad); }
body.wizard-mode .list, body.wizard-mode .top button { display: none; }

/* Badges */
.badge { display: inline-block; border-radius: 10px; padding: 1px 8px; font-size: 12px; font-weight: 600; color: #fff; white-space: nowrap; }
.badge.ok { background: var(--ok); } .badge.newer, .badge.changes { background: var(--warn); }
.badge.error { background: var(--bad); } .badge.new { background: var(--grey); }
.badge.wide { font-size: 13px; padding: 2px 10px; white-space: normal; }

/* Flavor page */
.head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.head h1 { font-size: 20px; }
.backups { display: grid; gap: 4px; }
.backup { display: flex; align-items: center; gap: 8px; padding: 6px 10px; border: 1px solid var(--line); border-radius: 6px; cursor: pointer; }
.backup.on { border-color: var(--accent); background: var(--sel); }
.confirm { border: 1px solid var(--line); border-radius: 8px; padding: 10px 12px; }
.files { margin: 6px 0; padding-left: 20px; max-height: 160px; overflow-y: auto; }

/* Settings */
.field { display: flex; align-items: center; gap: 8px; margin: 6px 0; }
.field > label { width: 110px; flex: none; color: var(--muted); }
.back { display: inline-block; margin-bottom: 6px; }
label.inline { display: inline-flex; align-items: center; gap: 8px; margin: 4px 16px 4px 0; }
.flavors { margin-top: 6px; }
input.switch { appearance: none; width: 30px; height: 18px; border-radius: 9px; background: var(--grey); position: relative;
  cursor: pointer; margin: 0; flex: none; }
input.switch::after { content: ""; position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%;
  background: #fff; transition: left .1s; }
input.switch:checked { background: var(--accent); }
input.switch:checked::after { left: 14px; }
.seg { display: inline-flex; border: 1px solid var(--line); border-radius: 6px; overflow: hidden; }
.seg label { padding: 4px 12px; cursor: pointer; }
.seg input { display: none; }
.seg label:has(input:checked) { background: var(--accent); color: #fff; }

/* Wizard */
.steps { display: flex; align-items: center; gap: 6px; margin-bottom: 18px; font-size: 13px; color: var(--muted); flex-wrap: wrap; }
.steps .n { width: 22px; height: 22px; border-radius: 50%; border: 1px solid var(--line); display: inline-flex;
  align-items: center; justify-content: center; font-size: 12px; margin-right: 4px; }
.steps .done .n { background: var(--ok); border-color: var(--ok); color: #fff; }
.steps .cur { color: var(--fg); font-weight: 600; }
.steps .cur .n { background: var(--accent); border-color: var(--accent); color: #fff; }
.steps .ln { flex: 1; min-width: 12px; height: 1px; background: var(--line); }
.choice { display: flex; gap: 10px; align-items: flex-start; border: 1px solid var(--line); border-radius: 8px; padding: 10px 12px; margin: 8px 0; cursor: pointer; }
.choice:has(input:checked) { border-color: var(--accent); background: var(--sel); }
.choice input { margin-top: 4px; }
ul.lines { padding-left: 20px; }

/* Conflict dialog */
body.dialog { padding: 16px 20px; }
table { border-collapse: collapse; width: 100%; }
td { padding: 7px 6px; border-bottom: 1px solid var(--line); vertical-align: middle; }
td:last-child { text-align: right; }
```

- [ ] **Step 2: Shared script with demo mode**

Replace `internal/ui/web/app.js` with:

```js
// Shared helpers for wtfsync's pages. Go methods live on ui.Bridge (internal/ui/bridge_windows.go).
// Opened with ?demo outside the app, pages use demo.js's sample data instead (for screenshots).
export const params = new URLSearchParams(location.search);
const demo = params.has("demo");
const rt = demo ? null : await import("/wails/runtime.js");
const fake = demo ? await import("./demo.js") : null;

const bridge = "github.com/rhwendt/wtfsync/internal/ui.Bridge.";

export const call = (method, ...args) => (demo ? fake.call(method, ...args) : rt.Call.ByName(bridge + method, ...args));
export const close = () => { if (!demo) rt.Window.Close(); };
export const on = (event, fn) => { if (!demo) rt.Events.On(event, (ev) => fn(ev.data)); };
export const openURL = (url) => call("OpenURL", url);

// applyTheme sets "light" or "dark", or follows Windows for "system".
export function applyTheme(appearance) {
  const t = params.get("theme") || appearance;
  if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
  else delete document.documentElement.dataset.theme;
}

// h builds an element: h("p", {class: "x"}, "text", child, [more]). Strings become text nodes;
// "" / null / false children are skipped.
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "checked" || k === "disabled" || k === "hidden") el[k] = !!v;
    else if (k === "value") el.value = v;
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false && v != null) el.setAttribute(k, v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === "" || c == null || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}

export const $ = (sel) => document.querySelector(sel);

export function errorText(e) {
  return (e && (e.message || e.toString())) || "Something went wrong";
}

export const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

// folderPicker is a <select> of options plus "Browse…".
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
  return { el: h("div", { class: "row grow" }, sel, browse), select: sel, value: () => sel.value };
}
```

- [ ] **Step 3: Demo data**

Create `internal/ui/web/demo.js`:

```js
// Sample data for screenshots: the same shapes ui.Bridge returns. Used only with ?demo.
const p = new URLSearchParams(location.search);

const main = {
  SetUp: true, Busy: "", PC: "beehive", Version: "v0.8.0", Update: "", Appearance: p.get("theme") || "system",
  Flavors: [
    { Name: "classic", Badge: { Kind: "ok", Short: "✓", Words: "In sync" },
      Line: "Last synced Oct 19, 02:07 · newest from leviathan",
      Backups: [{ ID: "b3", When: "Oct 19, 02:07", Reason: "before pull from leviathan", Files: 4 }], Missing: [] },
    { Name: "classic_beta", Badge: { Kind: "changes", Short: "↑ 26", Words: "26 changes to push" },
      Line: "Last synced today 03:51 · newest from leviathan",
      Backups: [
        { ID: "b2", When: "today 03:51", Reason: "before pull from leviathan", Files: 12 },
        { ID: "b1", When: "Oct 19, 02:07", Reason: "before pull from leviathan", Files: 4 },
        { ID: "b0", When: "Oct 18, 22:40", Reason: "before restore", Files: 4 },
      ],
      Missing: [{ PC: "leviathan", Addons: [
        { Name: "Details!", URL: "https://www.curseforge.com/projects/61284", Folders: "Details" },
        { Name: "Plater", URL: "https://www.curseforge.com/projects/100547", Folders: "Plater" },
      ] }] },
    { Name: "classic_era", Badge: { Kind: "new", Short: "new", Words: "Not synced yet" }, Line: "Never synced", Backups: [], Missing: [] },
    { Name: "retail", Badge: { Kind: "newer", Short: "↓", Words: "leviathan has newer settings" },
      Line: "Last synced Oct 18, 21:10 · leviathan pushed today 08:12", Backups: [], Missing: [] },
  ],
};

const answers = {
  Main: () => main,
  Settings: () => ({ Name: "beehive", WoWDir: "C:\\Program Files (x86)\\World of Warcraft", SyncFolder: "G:\\My Drive\\wtfsync",
    AllFlavors: false, StartAtLogin: true,
    Flavors: [{ Name: "classic", On: true }, { Name: "classic_beta", On: true }, { Name: "classic_era", On: false }, { Name: "retail", On: true }] }),
  Defaults: () => ({ Installs: ["C:\\Program Files (x86)\\World of Warcraft"], Clouds: ["G:\\My Drive\\wtfsync"],
    WoWDir: "C:\\Program Files (x86)\\World of Warcraft", Sync: "G:\\My Drive\\wtfsync", Name: "beehive", Rerun: false }),
  CloudPCs: () => ["leviathan"],
  Conflicts: () => ({ ID: 1, Other: "leviathan", FirstLink: false, Files: [
    "Account/WOWPLAYER/SavedVariables/ElvUI.lua",
    "Account/WOWPLAYER/SavedVariables/Details.lua",
    "Account/WOWPLAYER/Area 52/Thrall/SavedVariables/WeakAuras.lua",
  ] }),
  RestorePreview: () => ["Account/WOWPLAYER/SavedVariables/ElvUI.lua", "Account/WOWPLAYER/SavedVariables/Details.lua"],
  Restore: () => "b9",
  SaveSettings: () => ["Settings saved; sync history kept."],
  Setup: () => ["classic: got leviathan's settings (12 file(s))"],
};

export async function call(method) {
  return answers[method] ? answers[method]() : null;
}
```

- [ ] **Step 4: The main window page**

Create `internal/ui/web/main.html`:

```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>wtfsync</title></head>
<body class="app">
<header class="top">
  <span class="logo">WTF</span><span class="title">wtfsync</span><span id="busy" class="busy"></span>
  <span class="grow"></span>
  <button id="pullAll">Pull all</button><button id="pushAll">Push all</button><button id="gear" title="Settings">⚙</button>
</header>
<div class="body">
  <nav id="list" class="list"></nav>
  <main><div id="note"></div><div id="page"></div></main>
</div>
<script type="module" src="main.js"></script>
</body></html>
```

Create `internal/ui/web/main.js`:

```js
// The main window: flavor list, the selected flavor's page, Settings, and the setup wizard.
import { $, h, call, on, close, applyTheme, errorText, plural, openURL, params, folderPicker } from "./app.js";
import { wizard } from "./wizard.js";

const KEY = "wtfsync.flavor";
const remembered = () => { try { return localStorage.getItem(KEY) || ""; } catch { return ""; } };
const remember = (f) => { try { localStorage.setItem(KEY, f); } catch { /* not kept: fine */ } };

let data = null;                              // MainPage from Go
let view = params.get("view") || "flavor";    // "flavor", "settings" or "wizard"
let flavor = params.get("flavor") || remembered();
let settingsShown = false;                    // the settings form is built (rebuilding would lose edits)
let wizardShown = false;
const picked = {};                            // flavor → chosen backup id
let confirming = null;                        // {flavor, id, paths} while a restore is being confirmed
let note = null;                              // {text, error} shown above the page

const show = (...children) => $("#page").replaceChildren(...children);
const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

async function load() {
  try {
    data = await call("Main");
  } catch (e) {
    note = { text: errorText(e), error: true };
    return drawNote();
  }
  applyTheme(data.Appearance);
  if (!data.SetUp && view !== "wizard") view = "wizard";
  render();
}

// run does an action, shows its error (if any) above the page, then reloads.
async function run(fn) {
  note = null;
  try {
    await fn();
  } catch (e) {
    note = { text: errorText(e), error: true };
  }
  await load();
}

function drawNote() {
  $("#note").textContent = note ? note.text : "";
  $("#note").className = note && note.error ? "error" : "";
}

function render() {
  const busy = data.Busy !== "";
  $("#busy").textContent = busy ? cap(data.Busy) + "…" : "";
  $("#pullAll").disabled = $("#pushAll").disabled = busy || !data.SetUp;
  document.body.classList.toggle("wizard-mode", view === "wizard");
  drawNote();
  if (view === "wizard") return startWizard();
  wizardShown = false;
  renderList();
  if (view === "settings") return settingsPage();
  settingsShown = false;
  flavorPage();
}

function go(to) {
  view = to;
  note = null;
  confirming = null;
  render();
}

function renderList() {
  const fl = data.Flavors;
  if (!fl.some((f) => f.Name === flavor)) flavor = fl.length ? fl[0].Name : "";
  $("#list").replaceChildren(
    ...fl.map((f) => h("button", {
      class: "item" + (f.Name === flavor && view === "flavor" ? " on" : ""),
      onclick: () => { flavor = f.Name; remember(flavor); go("flavor"); },
    }, h("span", { class: "name" }, f.Name), h("span", { class: "badge " + f.Badge.Kind }, f.Badge.Short))),
    h("div", { class: "me" }, "This PC: ", h("b", {}, data.PC), h("br"), data.Version));
}

function flavorPage() {
  const f = data.Flavors.find((x) => x.Name === flavor);
  if (!f) return show(h("p", { class: "muted" }, "No flavors are being synced. Turn some on in Settings (⚙)."));
  const busy = data.Busy !== "";
  show(
    h("div", { class: "head" }, h("h1", {}, f.Name), h("span", { class: "grow" }),
      h("button", { disabled: busy, onclick: () => run(() => call("PullFlavor", f.Name)) }, "Pull"),
      h("button", { class: "primary", disabled: busy, onclick: () => run(() => call("PushFlavor", f.Name)) }, "Push")),
    h("div", {}, h("span", { class: "badge wide " + f.Badge.Kind }, f.Badge.Words)),
    h("p", { class: "muted" }, f.Line),
    h("h2", {}, "Backups"), backups(f, busy),
    h("h2", {}, "Missing addons"), missing(f));
}

function backups(f, busy) {
  if (confirming && confirming.flavor === f.Name) return confirmRestore(f);
  if (f.Backups.length === 0) return h("p", { class: "muted" }, "No backups yet. One is made each time a pull or restore changes files here.");
  if (!f.Backups.some((b) => b.ID === picked[f.Name])) picked[f.Name] = f.Backups[0].ID;
  return h("div", {},
    h("div", { class: "backups" }, f.Backups.map((b) =>
      h("label", { class: "backup" + (b.ID === picked[f.Name] ? " on" : "") },
        h("input", { type: "radio", name: "backup", checked: b.ID === picked[f.Name], onchange: () => { picked[f.Name] = b.ID; render(); } }),
        h("span", { class: "grow" }, b.When, b.Reason ? " · " + b.Reason : ""),
        h("span", { class: "muted" }, plural(b.Files, "file"))))),
    h("div", { class: "row" },
      h("span", { class: "muted grow" }, "Puts those files back as they were then. What's there now is backed up first."),
      h("button", { disabled: busy, onclick: () => run(async () => {
        const id = picked[f.Name];
        confirming = { flavor: f.Name, id, paths: await call("RestorePreview", f.Name, id) };
      }) }, "Restore…")));
}

function confirmRestore(f) {
  const c = confirming;
  const b = f.Backups.find((x) => x.ID === c.id);
  return h("div", { class: "confirm" },
    h("p", {}, `Restore ${plural(c.paths.length, "file")} as they were ${b ? b.When : "then"}?`),
    h("ul", { class: "files" }, c.paths.map((p) => h("li", {}, h("code", {}, p)))),
    h("div", { class: "row end" },
      h("button", { onclick: () => { confirming = null; render(); } }, "Cancel"),
      h("button", { class: "primary", disabled: data.Busy !== "", onclick: () => run(async () => {
        confirming = null;
        picked[f.Name] = await call("Restore", f.Name, c.id);
        note = { text: "Restored. What was there before is the backup now selected, so you can put it back." };
      }) }, "Restore")));
}

function missing(f) {
  if (f.Missing.length === 0) return h("p", { class: "muted" }, "None");
  return h("div", {}, f.Missing.map((g) => h("p", {}, `${g.PC} has: `,
    g.Addons.map((a, i) => [i ? " · " : "",
      a.URL ? h("a", { href: "#", title: a.Folders, onclick: (e) => { e.preventDefault(); openURL(a.URL); } }, a.Name)
            : h("span", { title: a.Folders }, a.Name)]))));
}

async function settingsPage() {
  if (settingsShown) return;
  settingsShown = true;
  let s;
  try {
    s = await call("Settings");
  } catch (e) {
    return show(h("p", { class: "error" }, errorText(e)));
  }
  const name = h("input", { type: "text", value: s.Name });
  const wow = folderPicker([], s.WoWDir, "Your World of Warcraft folder");
  const cloud = folderPicker([], s.SyncFolder, "A folder your cloud client syncs");
  const all = h("input", { type: "checkbox", class: "switch", checked: s.AllFlavors });
  const toggles = s.Flavors.map((t) => ({ name: t.Name, box: h("input", { type: "checkbox", class: "switch", checked: t.On }) }));
  const each = h("div", { class: "flavors", hidden: s.AllFlavors }, toggles.map((t) => h("label", { class: "inline" }, t.box, t.name)));
  all.addEventListener("change", () => { each.hidden = all.checked; });
  const login = h("input", { type: "checkbox", class: "switch", checked: s.StartAtLogin });
  const theme = h("div", { class: "seg" }, [["system", "System"], ["light", "Light"], ["dark", "Dark"]].map(([v, label]) =>
    h("label", {}, h("input", { type: "radio", name: "theme", checked: data.Appearance === v, onchange: () => {
      applyTheme(v);
      call("SetAppearance", v).catch((e) => { note = { text: errorText(e), error: true }; drawNote(); });
    } }), label)));
  const field = (label, ...el) => h("div", { class: "field" }, h("label", {}, label), ...el);
  const about = data.Update ? `wtfsync ${data.Version} · ${data.Update} available` : `wtfsync ${data.Version} · up to date`;
  const save = h("button", { class: "primary", onclick: () => run(async () => {
    const lines = await call("SaveSettings", {
      Name: name.value, WoWDir: wow.value(), SyncFolder: cloud.value(), AllFlavors: all.checked,
      Flavors: toggles.map((t) => ({ Name: t.name, On: t.box.checked })), StartAtLogin: login.checked,
    });
    note = { text: lines.join("\n") || "Saved." };
    settingsShown = false; // show what was saved
  }) }, "Save");
  show(
    h("a", { href: "#", class: "back", onclick: (e) => { e.preventDefault(); go("flavor"); } }, "← Back"),
    h("h1", {}, "Settings"),
    h("h2", {}, "This PC"), field("Name", name),
    h("h2", {}, "Folders"), field("WoW folder", wow.el), field("Cloud folder", cloud.el),
    h("h2", {}, "Flavors"), h("label", { class: "inline" }, all, "All flavors, including ones installed later"), each,
    h("h2", {}, "Appearance"), theme,
    h("h2", {}, "Startup"), h("label", { class: "inline" }, login, "Start wtfsync when Windows starts"),
    h("h2", {}, "About"),
    h("div", { class: "row" }, h("span", { class: "grow" }, about),
      h("button", { onclick: () => run(async () => { await call("CheckUpdate"); settingsShown = false; }) }, "Check for updates"),
      data.Update ? h("button", { class: "primary", onclick: () => call("InstallUpdate") }, "Update to " + data.Update) : "",
      h("button", { onclick: () => call("OpenLogFolder") }, "Open log folder")),
    h("div", { class: "row end" },
      h("button", { onclick: () => go("wizard") }, "Run setup again…"), h("span", { class: "grow" }),
      h("button", { onclick: () => go("flavor") }, "Cancel"), save));
}

function startWizard() {
  if (wizardShown) return;
  wizardShown = true;
  const from = data.SetUp ? (settingsShown ? "settings" : "flavor") : null;
  settingsShown = false;
  wizard($("#page"), {
    start: Math.max(0, Number(params.get("step") || 1) - 1),
    done: () => { view = "flavor"; load(); },
    // On a PC that isn't set up, Cancel just hides the window; the tray still offers Set up.
    cancel: from ? () => { view = from; load(); } : () => close(),
  });
}

$("#pullAll").onclick = () => run(() => call("PullAll"));
$("#pushAll").onclick = () => run(() => call("PushAll"));
$("#gear").onclick = () => go("settings");

let pending = null; // coalesce bursts of "changed" (a sync emits several)
on("changed", () => { clearTimeout(pending); pending = setTimeout(load, 150); });
on("show", (v) => { if (v) go(v); else load(); });
await load();
```

- [ ] **Step 5: The wizard**

Create `internal/ui/web/wizard.js`:

```js
// The setup wizard, drawn into the main window's page area.
import { h, call, errorText, folderPicker } from "./app.js";

const STEPS = ["WoW folder", "Cloud folder", "PC name", "First PC or join", "Sync"];

// wizard runs setup inside root. done() runs after Finish; cancel() on Cancel.
export async function wizard(root, { start = 0, done, cancel }) {
  const d = await call("Defaults");
  const wow = folderPicker(d.Installs, d.WoWDir, "Your World of Warcraft folder");
  const cloud = folderPicker(d.Clouds, d.Sync, "A folder your cloud client syncs");
  const name = h("input", { type: "text", value: d.Name });
  let pcs = [];          // PCs already in the chosen cloud folder
  let first = true;      // "This is my first PC"
  let step = Math.min(start, 3);
  let error = "";
  let result = null;     // lines from Setup, on the last step

  const lookUp = async () => { pcs = await call("CloudPCs", cloud.value()); first = pcs.length === 0; };
  if (step >= 3) await lookUp();

  async function next() {
    error = "";
    if (step === 1) await lookUp();
    if (step === 2 && !name.value.trim()) { error = "This PC needs a name."; return draw(); }
    if (step === 3) return sync();
    step++;
    draw();
  }

  async function sync() {
    step = 4;
    result = null;
    draw();
    try {
      result = await call("Setup", { WoWDir: wow.value(), SyncFolder: cloud.value(), Name: name.value.trim(), First: first });
    } catch (e) {
      error = errorText(e);
      step = 3;
    }
    draw();
  }

  const choice = (value, title, sub) => h("label", { class: "choice" },
    h("input", { type: "radio", name: "mode", checked: first === value, onchange: () => { first = value; } }),
    h("div", {}, h("b", {}, title), h("div", { class: "muted" }, sub)));

  function body() {
    switch (step) {
      case 0: return [h("h1", {}, "Where is World of Warcraft installed?"),
        h("p", { class: "muted" }, "The folder that holds _retail_, _classic_ and the other flavors."), wow.el];
      case 1: return [h("h1", {}, "Which cloud folder should wtfsync use?"),
        h("p", { class: "muted" }, "A folder Google Drive, OneDrive, Dropbox or iCloud keeps in sync on all your PCs."), cloud.el];
      case 2: return [h("h1", {}, "What should this PC be called?"),
        h("p", { class: "muted" }, "Your other PCs show this name."), name];
      case 3: return pcs.length
        ? [h("h1", {}, "Is this your first PC with wtfsync?"),
          h("p", { class: "muted" }, "This cloud folder already has settings from ", h("b", {}, pcs.join(", ")), "."),
          choice(true, "This is my first PC", "Start a new sync with this PC's settings."),
          choice(false, "Join " + pcs.join(", "), "Get their settings. If both PCs changed the same file, you'll choose which to keep.")]
        : [h("h1", {}, "Start syncing from this PC"),
          h("p", { class: "muted" }, "This cloud folder has no wtfsync data yet, so this PC starts it. Set up your other PCs afterwards and choose to join this one.")];
      default: return result
        ? [h("h1", {}, d.Rerun ? "Saved" : "This PC is set up"), h("ul", { class: "lines" }, result.map((l) => h("li", {}, l)))]
        : [h("h1", {}, "Setting up…"), h("p", { class: "muted" }, "The first sync can take a minute.")];
    }
  }

  function buttons() {
    if (step === 4) return result ? [h("span", { class: "grow" }), h("button", { class: "primary", onclick: done }, "Finish")] : [];
    return [
      step > 0 ? h("button", { onclick: () => { error = ""; step--; draw(); } }, "← Back") : "",
      h("span", { class: "grow" }),
      h("button", { onclick: cancel }, "Cancel"),
      h("button", { class: "primary", onclick: next }, step === 3 ? "Set up" : "Next"),
    ];
  }

  function draw() {
    root.replaceChildren(
      h("div", { class: "steps" }, STEPS.map((s, i) => [
        i ? h("span", { class: "ln" }) : "",
        h("span", { class: i < step ? "done" : i === step ? "cur" : "" }, h("span", { class: "n" }, i < step ? "✓" : String(i + 1)), s),
      ])),
      body(),
      error ? h("p", { class: "error" }, error) : "",
      h("div", { class: "row end" }, buttons()));
  }

  draw();
}
```

- [ ] **Step 6: The conflict dialog**

Replace `internal/ui/web/conflicts.html` with:

```html
<!doctype html>
<html><head><meta charset="utf-8"><link rel="stylesheet" href="app.css"><title>Settings changed on both PCs</title></head>
<body class="dialog"><div id="main"></div>
<script type="module" src="conflicts.js"></script>
</body></html>
```

Create `internal/ui/web/conflicts.js`:

```js
// The conflict dialog: one window per question; closing it cancels that question (ui.openConflicts).
import { $, h, call, close, applyTheme } from "./app.js";

const show = (...children) => $("#main").replaceChildren(...children);
applyTheme((await call("Main"))?.Appearance || "system");
const req = await call("Conflicts");
// Answer before closing: the window's close event cancels this question if it's still open.
const reply = async (choices, cancel) => { await call("AnswerConflicts", req.ID, choices, cancel); close(); };
const all = (c) => Object.fromEntries(req.Files.map((f) => [f, c]));
const footer = h("p", { class: "muted" }, "Cancel changes nothing. Files you replace are backed up first.");

function perFile() {
  const rows = req.Files.map((f) => {
    const pick = { value: "remote" };
    const seg = h("span", { class: "seg" }, [["remote", `${req.Other}'s`], ["local", "this PC's"]].map(([v, label]) =>
      h("label", {}, h("input", { type: "radio", name: f, checked: v === "remote", onchange: () => { pick.value = v; } }), label)));
    return { f, pick, seg, tr: h("tr", {}, h("td", {}, h("code", {}, f)), h("td", {}, seg)) };
  });
  const setAll = (v) => rows.forEach((r) => {
    r.pick.value = v;
    r.seg.querySelectorAll("input").forEach((i, n) => { i.checked = (n === 0) === (v === "remote"); });
  });
  show(
    h("h1", {}, "Settings changed on both PCs"),
    h("p", {}, `These files changed on `, h("b", {}, req.Other), ` and on this PC since you last synced. Choose which version to keep.`),
    h("table", {}, rows.map((r) => r.tr)),
    h("div", { class: "row" }, h("span", { class: "muted" }, "Set all to"),
      h("button", { onclick: () => setAll("remote") }, `${req.Other}'s`),
      h("button", { onclick: () => setAll("local") }, "this PC's"),
      h("span", { class: "grow" }),
      h("button", { onclick: () => reply(null, true) }, "Cancel"),
      h("button", { class: "primary", onclick: () => reply(Object.fromEntries(rows.map((r) => [r.f, r.pick.value])), false) }, "Apply")),
    footer);
}

if (!req.Files || req.Files.length === 0) {
  show(h("p", { class: "muted" }, "Nothing to decide."), h("div", { class: "row end" }, h("button", { onclick: close }, "Close")));
} else if (req.FirstLink) {
  show(
    h("h1", {}, "Settings changed on both PCs"),
    h("p", {}, `This PC and `, h("b", {}, req.Other), ` both have settings for ${req.Files.length} file(s).`),
    h("div", { class: "row end" },
      h("button", { onclick: () => reply(null, true) }, "Cancel"),
      h("button", { onclick: perFile }, "Choose per file…"),
      h("button", { onclick: () => reply(all("local"), false) }, "Keep this PC's"),
      h("button", { class: "primary", onclick: () => reply(all("remote"), false) }, `Use ${req.Other}'s for all`)),
    footer);
} else {
  perFile();
}
```

Delete the old pages:

```bash
git rm internal/ui/web/status.html internal/ui/web/restore.html internal/ui/web/addons.html internal/ui/web/settings.html internal/ui/web/wizard.html
```

- [ ] **Step 7: Check the scripts parse**

Run: `for f in internal/ui/web/*.js; do node --check "$f" || echo "FAIL $f"; done`
Expected: no output (Node 22+ detects ES module syntax). If your Node rejects `import`/top-level `await` as CommonJS, run `cp "$f" /tmp/x.mjs && node --check /tmp/x.mjs` per file instead; still expect no output.

- [ ] **Step 8: The screenshot script**

Create `.github/scripts/screenshots.sh` (and `chmod +x` it):

```bash
#!/usr/bin/env bash
# Renders the tray app's pages with sample data (demo mode, see internal/ui/web/demo.js) into PNGs.
# usage: screenshots.sh [out-dir]    (default docs/screenshots)
# Needs python3 and Chrome or Chromium; set CHROME to its path if it isn't on PATH.
set -euo pipefail

out=$(realpath -m "${1:-docs/screenshots}")
chrome=${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}
if [ -z "$chrome" ]; then
  echo "screenshots.sh: needs google-chrome or chromium (or CHROME=/path/to/chrome)" >&2
  exit 1
fi
mkdir -p "$out"

port=8765
python3 -m http.server "$port" --bind 127.0.0.1 --directory internal/ui/web >/dev/null 2>&1 &
server=$!
trap 'kill $server' EXIT
for _ in $(seq 50); do
  curl -fs "http://127.0.0.1:$port/main.html" >/dev/null && break
  sleep 0.1
done

shot() { # name, page and query, width, height
  "$chrome" --headless --disable-gpu --no-first-run --hide-scrollbars --force-device-scale-factor=2 \
    --window-size="$3,$4" --virtual-time-budget=3000 \
    --screenshot="$out/$1.png" "http://127.0.0.1:$port/$2" >/dev/null 2>&1
}

shot main "main.html?demo&flavor=classic_beta&theme=light" 1000 640
shot main-dark "main.html?demo&flavor=classic_beta&theme=dark" 1000 640
shot settings "main.html?demo&view=settings&theme=light" 1000 760
shot wizard "main.html?demo&view=wizard&step=4&theme=light" 1000 640
shot conflicts "conflicts.html?demo&theme=light" 760 460
echo "wrote $(ls "$out"/*.png | wc -l) screenshots to $out"
```

- [ ] **Step 9: Render the pages and look at them**

If no Chrome is installed locally, fetch a headless one into a scratch folder:

```bash
npx -y @puppeteer/browsers install chrome-headless-shell@stable --path "$HOME/.cache/wtfsync-chrome"
export CHROME=$(ls -d "$HOME"/.cache/wtfsync-chrome/chrome-headless-shell/*/chrome-headless-shell-linux64/chrome-headless-shell)
```

Run: `.github/scripts/screenshots.sh /tmp/wtfsync-shots`
Expected: `wrote 5 screenshots to /tmp/wtfsync-shots`.

Open each PNG (Read tool) and check against the approved mockups: top bar with Pull all / Push all / ⚙; flavor list with `✓`, `↑ 26`, `new`, `↓` badges; classic_beta page with an amber "26 changes to push" badge, three backups with the first selected, and two linked missing addons; the dark shot uses the dark palette everywhere; Settings shows the flavor switches (All off), the System/Light/Dark control and the About row; the wizard is on step 4 with "Join leviathan" offered; the conflict dialog shows three rows with **leviathan's | this PC's** toggles. Nothing is cut off or overlapping. Fix any page that doesn't match before committing.

- [ ] **Step 10: Commit**

```bash
git add internal/ui/web .github/scripts/screenshots.sh
git commit -m "feat: add the main window's pages (flavor list, flavor page, settings, wizard)"
```

---

### Task 7: Wire the main window into the tray

**Files:**
- Modify: `internal/ui/ui_windows.go`, `internal/ui/bridge_windows.go`
- Modify: `internal/app/present.go`, `internal/app/present_test.go` (remove `StatusRows`, `AddonRows`)

**Interfaces:**
- Consumes: everything listed under Task 6's "Consumes"; `app.App.Main`, `PushFlavor`, `PullFlavor`, `RestorePreview(flavor, id)`, `Restore(flavor, id)`, `SetAppearance`, `CloudPCs`, `CheckUpdate` (Task 5); `local.Settings.Theme` (Task 1); `app.SettingsInput.AllFlavors` (Task 1).
- Produces: Wails window `main` (`/main.html`), events `changed` and `show`; Bridge methods exactly as named in Task 6.

- [ ] **Step 1: Remove the old page models**

In `internal/app/present.go`, delete `StatusRow`, `StatusRows`, `AddonRow` and `AddonRows` (keep `ResultLines`). In `internal/app/present_test.go`, delete `TestStatusRows`, `TestAddonRows` and the empty-list checks for them in `TestEmptyRowsAreEmptyLists` (keep its `ResultLines` check). Remove imports `go vet` then reports as unused.

Run: `go test ./internal/app/`
Expected: ok.

- [ ] **Step 2: Rewrite the Bridge**

Replace everything in `internal/ui/bridge_windows.go` from `type StatusPage struct` to the end of the file with:

```go
// Main is the main window's data.
func (b *Bridge) Main() app.MainPage { return b.u.app.Main() }

func (b *Bridge) PushAll() error              { return b.u.app.Push() }
func (b *Bridge) PullAll() error              { return b.u.app.Pull() }
func (b *Bridge) PushFlavor(name string) error { return b.u.app.PushFlavor(name) }
func (b *Bridge) PullFlavor(name string) error { return b.u.app.PullFlavor(name) }

// RestorePreview lists the files restoring backup id of flavor would rewrite or remove.
func (b *Bridge) RestorePreview(flavor, id string) ([]string, error) {
	r, err := b.u.app.RestorePreview(flavor, id)
	return list(r.Paths), err
}

// Restore puts backup id of flavor back and returns the id of the backup of what it replaced.
func (b *Bridge) Restore(flavor, id string) (string, error) {
	r, err := b.u.app.Restore(flavor, id)
	return r.Undo, err
}

func (b *Bridge) SetAppearance(v string) error { return b.u.app.SetAppearance(v) }
func (b *Bridge) CheckUpdate() error           { return b.u.app.CheckUpdate(context.Background()) }
func (b *Bridge) InstallUpdate()               { go b.u.installUpdate() }
func (b *Bridge) OpenLogFolder()               { b.u.openLogFolder() }
func (b *Bridge) CloudPCs(folder string) []string { return list(b.u.app.CloudPCs(folder)) }

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
	AllFlavors   bool
	Flavors      []FlavorToggle // every flavor in the WoW folder; used when AllFlavors is off
	StartAtLogin bool
}

func (b *Bridge) Settings() (SettingsForm, error) {
	s, ok, err := b.u.app.Settings()
	if err != nil || !ok {
		return SettingsForm{}, cmp.Or(err, errors.New("this PC isn't set up yet"))
	}
	f := SettingsForm{Name: s.MachineName, WoWDir: s.WoWDir, SyncFolder: s.SyncFolder, AllFlavors: s.All(),
		StartAtLogin: b.u.app.StartsAtLogin(), Flavors: []FlavorToggle{}}
	flavors, _ := setup.Flavors(s.WoWDir)
	for _, fl := range flavors {
		f.Flavors = append(f.Flavors, FlavorToggle{Name: fl.Name, On: s.Enabled(fl.Name)})
	}
	slices.SortFunc(f.Flavors, func(a, b FlavorToggle) int { return strings.Compare(a.Name, b.Name) })
	return f, nil
}

func (b *Bridge) SaveSettings(f SettingsForm) ([]string, error) {
	in := app.SettingsInput{Name: f.Name, WoWDir: f.WoWDir, SyncFolder: f.SyncFolder, AllFlavors: f.AllFlavors,
		StartAtLogin: f.StartAtLogin, Flavors: map[string]bool{}}
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
	return list(lines), nil
}
```

Also delete `Bridge.HasSyncData` (the wizard uses `CloudPCs` now).

- [ ] **Step 3: The main window and tray click**

In `internal/ui/ui_windows.go`:

Add a field to `UI`:

```go
	quitting atomic.Bool // set by quit, so closing the main window then really closes it
```

In `Run`, right after `u.tray = u.wails.SystemTray.New()`:

```go
	u.tray.OnClick(func() { u.showMain("") }) // right-click still shows the menu
```

In `loop`, replace

```go
	if !u.app.View().SetUp {
		u.open("wizard")
	}
```

with

```go
	if !u.app.View().SetUp {
		u.showMain("wizard")
	}
```

At the end of `render`, after `u.tray.SetMenu(menu)`:

```go
	u.wails.Event.Emit("changed") // the main window reloads its data
```

Replace the switch in `do` with:

```go
	switch id {
	case "push":
		go u.app.Push()
	case "pull":
		go u.app.Pull()
	case "update":
		go u.installUpdate()
	case "quit":
		go u.quit()
	case "open", "addons":
		u.showMain("")
	case "setup":
		u.showMain("wizard")
	case "settings":
		u.showMain("settings")
	case "logs":
		u.openLogFolder()
	}
```

In `quit`, make the first line:

```go
	u.quitting.Store(true)
```

Replace `secondInstance`'s body after the `--quit` check with:

```go
	u.showMain("")
```

Replace the `page` type, the `pages` map, `open`, `openConflicts` and `newWindow` with:

```go
// open is how the conflict prompter asks for its dialog.
func (u *UI) open(name string) {
	if name == "conflicts" {
		u.openConflicts(u.prompt.request().ID)
	}
}

// showMain opens the main window, or brings it to the front, on view ("" keeps what it shows;
// "settings" or "wizard"). Closing it only hides it. Each time it's shown, status is refreshed.
func (u *UI) showMain(view string) {
	go u.app.Refresh()
	application.InvokeAsync(func() {
		if w, ok := u.wails.Window.GetByName("main"); ok {
			u.wails.Event.Emit("show", view)
			w.Show()
			w.Focus()
			return
		}
		url := "/main.html"
		if view != "" {
			url += "?view=" + view
		}
		w := u.newWindow("main", "wtfsync", 860, 560, url)
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if u.quitting.Load() {
				return
			}
			e.Cancel()
			w.Hide()
		})
		w.Show()
	})
}

// openConflicts opens one conflict question's dialog; closing it cancels only that question.
func (u *UI) openConflicts(id int) {
	application.InvokeAsync(func() {
		w := u.newWindow(fmt.Sprintf("conflicts-%d", id), "Settings changed on both PCs", 680, 480, fmt.Sprintf("/conflicts.html?id=%d", id))
		w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.prompt.reply(id, nil, true) })
		w.Show()
	})
}

// newWindow makes one of the tray's windows, logging when it opens and closes. Its background
// matches the appearance setting, so a page that hasn't loaded (or failed to) isn't a black box.
func (u *UI) newWindow(name, title string, width, height int, url string) *application.WebviewWindow {
	s, _, _ := u.app.Settings()
	theme := s.Theme()
	bg := application.NewRGB(0xff, 0xff, 0xff)
	if theme == "dark" || (theme == "system" && u.wails.Env.IsDarkMode()) {
		bg = application.NewRGB(0x1f, 0x1f, 0x1f)
	}
	u.opts.Log.Printf("window: open %s", name)
	w := u.wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: name, Title: title, Width: width, Height: height, MinWidth: 640, MinHeight: 420, URL: url, BackgroundColour: bg,
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { u.opts.Log.Printf("window: closed %s", name) })
	return w
}
```

- [ ] **Step 4: Build and vet for Windows**

Run: `go test ./... && GOOS=windows go vet ./... && GOOS=windows CGO_ENABLED=0 go build -ldflags "-H windowsgui" -o /dev/null ./cmd/wtfsync-tray && echo WINOK`
Expected: all tests ok, vet silent, `WINOK`. Remove any import vet reports as unused.

- [ ] **Step 5: Check the page and the Bridge agree**

Run: `grep -oh 'call("[A-Za-z]*"' internal/ui/web/*.js | sort -u | sed 's/call("//;s/"//' | while read m; do grep -q "func (b \*Bridge) $m(" internal/ui/bridge_windows.go || echo "MISSING $m"; done`
Expected: no output (every method a page calls exists on the Bridge).

- [ ] **Step 6: Commit**

```bash
git add internal/ui internal/app
git commit -m "feat: open the main window from the tray and drop the old windows"
```

---

### Task 8: README screenshots, CLI reference, and CI screenshots

**Files:**
- Create: `docs/screenshots/*.png` (generated)
- Modify: `README.md`
- Create: `internal/cli/readme_test.go`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `.github/scripts/screenshots.sh` (Task 6); the CLI's `usage` constant (`internal/cli/cli.go`).
- Produces: README sections "Command reference" and an updated "Tray app"; `TestReadmeDocumentsEveryCommandAndFlag`.

- [ ] **Step 1: Write the failing test**

Create `internal/cli/readme_test.go`:

```go
package cli

import (
	"os"
	"strings"
	"testing"
)

// usageNames lists the commands and flags in the CLI's usage text.
func usageNames(u string) (cmds, flags []string) {
	inCmds := false
	for _, line := range strings.Split(u, "\n") {
		switch {
		case line == "commands:":
			inCmds = true
		case strings.TrimSpace(line) == "":
			inCmds = false
		case strings.HasPrefix(line, "  --"):
			flags = append(flags, strings.Fields(line)[0])
		case inCmds && strings.HasPrefix(line, "  "):
			cmds = append(cmds, strings.Fields(line)[0])
		}
	}
	return cmds, flags
}

func TestUsageNamesFindsEverything(t *testing.T) {
	cmds, flags := usageNames(usage)
	if len(cmds) != 7 || cmds[0] != "init" || len(flags) != 7 || flags[0] != "--flavor" {
		t.Fatalf("commands %v, flags %v", cmds, flags)
	}
}

func TestReadmeDocumentsEveryCommandAndFlag(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, ref, ok := strings.Cut(string(b), "\n## Command reference\n")
	if !ok {
		t.Fatal(`README has no "## Command reference" section`)
	}
	if end := strings.Index(ref, "\n## "); end >= 0 {
		ref = ref[:end]
	}
	cmds, flags := usageNames(usage)
	for _, c := range cmds {
		if !strings.Contains(ref, "### `wtfsync "+c+"`") {
			t.Errorf("Command reference has no ### `wtfsync %s` heading", c)
		}
	}
	for _, f := range flags {
		if !strings.Contains(ref, "`"+f) {
			t.Errorf("Command reference doesn't mention %s", f)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/cli/ -run 'Readme|UsageNames'`
Expected: `TestUsageNamesFindsEverything` passes; `TestReadmeDocumentsEveryCommandAndFlag` FAILS with `README has no "## Command reference" section`.

- [ ] **Step 3: Generate the screenshots**

Run (with `CHROME` set as in Task 6 Step 9 if needed): `.github/scripts/screenshots.sh`
Expected: `wrote 5 screenshots to …/docs/screenshots`. Look at each PNG once more.

- [ ] **Step 4: Write the README sections**

In `README.md`, insert right after the first paragraph (after "…(Google Drive for Desktop, OneDrive, iCloud, Dropbox).") :

```markdown
![The wtfsync window: flavors on the left, the selected flavor's status, backups and missing addons on the right](docs/screenshots/main.png)
```

Replace the whole `## Tray app` section (from `## Tray app` up to, not including, `**Something wrong?**`) with:

```markdown
## Tray app

`wtfsync-tray.exe` (the Start menu entry) puts a **WTF** icon in the notification area. The strip
along its bottom shows the state:

| Strip | Means |
|---|---|
| green ✓ | every flavor is in sync |
| amber ↓ | another PC pushed newer settings (a notification offers **Pull**) |
| red ! | needs you: an error, a refused sync (close WoW first), a push waiting for the cloud folder, or missing addons |
| blue ⟳ | syncing |

Click the icon to open the wtfsync window; right-click for the menu (Open wtfsync, Push, Pull,
Settings, Update, Open log folder, Quit). Closing the window keeps wtfsync running in the tray.

In the window, pick a WoW flavor on the left to see everything about it: whether it's in sync and
when it last synced, **Pull** and **Push** for just that flavor, its backups (pick one and
**Restore…** — what's there now is backed up first, so a restore can be undone), and addons your
other PCs have that this one doesn't, with download links. **Pull all** and **Push all** are in the
top bar.

**⚙ Settings** has this PC's name, the WoW and cloud folders, which flavors to sync (**All
flavors**, or pick them), Appearance (System, Light or Dark), start with Windows, updates, and
**Run setup again**.

| Settings | Setup |
|---|---|
| ![Settings](docs/screenshots/settings.png) | ![Setup wizard](docs/screenshots/wizard.png) |

If the same addon's settings changed on both PCs, a window asks which version to keep, file by
file; Cancel changes nothing.

![Conflict dialog](docs/screenshots/conflicts.png)

It follows Windows' light or dark mode unless you pick one in Settings:

![The window in dark mode](docs/screenshots/main-dark.png)

The tray and the `wtfsync` command never sync at the same time: whichever starts second says "a
sync is already running".

```

Insert a new section before `## Build`:

```markdown
## Command reference

`wtfsync.exe` is the command line: everything the tray does, scriptable. Close WoW before syncing;
commands that would change files refuse while that flavor's game is running.

### `wtfsync init`

Sets up the first PC: asks for the cloud folder, the WoW folder and a name for this PC, creates
wtfsync's data in the cloud folder, then pushes. Run it again to change any of those answers; the
PC keeps its identity, and pointing it at a different folder clears its sync history.

    wtfsync init
    wtfsync init --sync-dir "G:\My Drive" --name desktop

### `wtfsync link`

Sets up another PC on a cloud folder that already has wtfsync data, then pulls. On a first link,
when both PCs have settings, it asks once whether to use the other PC's for everything, keep this
PC's, or choose per file.

    wtfsync link

### `wtfsync push`

Uploads this PC's addon settings for every enabled flavor as a new snapshot. If another PC pushed
since this one last synced, its changes are merged first. If the cloud folder can't be reached,
the push is kept pending and retried by the next command (or the tray, every minute).

    wtfsync push
    wtfsync push --flavor classic

### `wtfsync pull`

Downloads the newest settings from your other PCs. Files changed only there are taken, files
changed only here are kept, and files changed on both ask which to keep. Every file it overwrites
or deletes is backed up first.

    wtfsync pull
    wtfsync pull --dry-run

### `wtfsync status`

Shows, per flavor, when it last synced, the newest snapshot and who pushed it, how many files
changed here, whether a push is pending, and addons other PCs have that this one doesn't.

    wtfsync status

### `wtfsync restore`

Puts back the files from the last backup (or an older one with `--set`), backing up what it
replaces first. It lists the other backups you can pick from.

    wtfsync restore --flavor retail
    wtfsync restore --flavor retail --set 20261020T035114.000Z

### `wtfsync version`

Prints the version.

### Flags

| Flag | Applies to | Meaning |
|---|---|---|
| `--flavor <name>` | push, pull, status, restore | only this flavor (`retail`, `classic`, …); default: every enabled flavor |
| `--dry-run` | all but version | show what would change; change nothing |
| `--wow-dir <path>` | all but version | WoW install folder (overrides settings) |
| `--sync-dir <path>` | all but version | the cloud-synced folder (overrides settings) |
| `--data-dir <path>` | all but version | wtfsync's local data folder (default `%APPDATA%\wtfsync`) |
| `--name <name>` | init, link | this PC's display name |
| `--set <id>` | restore | which backup to restore (ids are listed by `restore`) |

### Exit codes

| Code | Means |
|---|---|
| 0 | done |
| 1 | error |
| 2 | refused: WoW is running, a snapshot isn't downloaded from the cloud yet, or another sync is running |
| 3 | a conflict question was cancelled; nothing was changed |

```

In the `## Manual test checklist (real PCs)` list, replace items 5, 7 and 10 and append new ones so the list reads (keep items 1–4, 6, 8, 9, 11, 12 as they are):

```markdown
5. Install `wtfsync-setup.exe`: Start menu entry, tray icon, and the window opening on the setup wizard on a PC that isn't set up.
7. Same-file conflict → the conflict window; Cancel leaves files as they were; it appears even while the main window is hidden.
10. Settings: rename the PC, All flavors off then a flavor off, start at login (check after a reboot).
13. Tray left-click opens and focuses the window; closing it hides it; Quit from the menu really quits.
14. Badges on both PCs match reality: ✓, ↑ N after editing a file, ↓ after the other PC pushes, `new` for a flavor never synced.
15. Restore an older backup (not the newest), then restore the "before restore" backup to undo it.
16. Appearance Light / Dark / System, including Windows switching theme while the window is open.
```

- [ ] **Step 5: Run the README test**

Run: `go test ./internal/cli/ -run 'Readme|UsageNames' -v`
Expected: both PASS.

- [ ] **Step 6: Screenshots in CI**

In `.github/workflows/ci.yml`, in the `build` job, add after the `upload-artifact` step:

```yaml
      # PRs that change the pages get fresh screenshots to review (docs/screenshots is refreshed by
      # hand with the same script).
      - name: Screenshots of the pages
        id: shots
        if: github.event_name == 'pull_request'
        run: |
          git fetch -q --depth=1 origin "${{ github.base_ref }}"
          if git diff --quiet FETCH_HEAD -- internal/ui/web; then
            echo "pages unchanged"
          else
            .github/scripts/screenshots.sh dist/screenshots
            echo "made=true" >> "$GITHUB_OUTPUT"
          fi
      - uses: actions/upload-artifact@v7
        if: steps.shots.outputs.made == 'true'
        with:
          name: screenshots
          path: dist/screenshots
```

- [ ] **Step 7: Run everything**

Run: `go test ./... && GOOS=windows go vet ./... && GOOS=windows CGO_ENABLED=0 go build -o /dev/null ./cmd/wtfsync-tray && echo OK`
Expected: all ok, `OK`.

- [ ] **Step 8: Commit**

```bash
git add README.md docs/screenshots internal/cli/readme_test.go .github/workflows/ci.yml
git commit -m "docs: show the window in the README and add a command reference"
```

After the PR is open, check that the `build` job uploaded a `screenshots` artifact (the pages changed in this branch) and that its PNGs match `docs/screenshots`.
