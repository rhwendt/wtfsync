# One Sync button and restore points — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Push/Pull with one Sync, and the window's local Backups list with restore points
taken from the cloud folder's kept snapshots, so every PC can go back to any recent sync.

**Architecture:** `engine.Status` already opens the sync folder on every refresh; it also lists the
flavor's snapshots as restore points and marks the current one from local state (`FlavorState`
gains `Restored`/`RestoredFiles`). A new `engine.RestoreSnapshot` makes the synced files equal a
snapshot, backing up what it replaces. `app` turns those into page rows and replaces
Push/Pull with Sync (= `engine.Push`); `ui` and `web` follow; the CLI gains `sync`.

**Tech Stack:** Go 1.26, Wails v3.0.0-beta.32, plain ES-module pages.

**Spec:** `docs/superpowers/specs/2026-10-22-sync-and-restore-points-design.md`

## Global Constraints

- Restore points: every snapshot kept in the cloud folder (20 per flavor, unchanged), newest first.
- Rows show the time (`when()` wording) and "from <PC>"; no file counts.
- Current row: `restored` when local files hash to `restoredFiles`; else `base` when no local changes; else none.
- Restore never touches files `collect.Synced` rejects; backs up what it overwrites or deletes (reason "restore").
- Local backups: still made, still 10, not shown in the window. CLI `restore` unchanged.
- UI words: "sync", not "push"/"pull". CLI keeps `push`/`pull`, adds `sync` (= push).
- Commits: Conventional Commits; never on main.

## Review Focus

1. Restore of a snapshot with fewer files than local: extra synced files are deleted, unsynced files (e.g. `Config.wtf`) are untouched.
2. Restore then Sync while another PC pushed meanwhile: the merge sees local changes, conflicts are asked, `restored*` cleared.
3. A restore-point name from the page that isn't a kept snapshot (pruned, or crafted): error, nothing written, no path escapes.
4. A state file written by an older version (no `restored*`): reads as not restored.
5. Restoring the snapshot the files already match (no changes): nothing rewritten, no empty backup set noise beyond what Backup makes.

---

### Task 1: State records a restore

**Files:** Modify `internal/local/store.go` (FlavorState). Test: `internal/local/store_test.go`.

**Produces:** `FlavorState.Restored string` (`json:"restored,omitempty"`), `FlavorState.RestoredFiles map[string]string` (`json:"restoredFiles,omitempty"`).

- [ ] Step 1: test `TestStateRestoredRoundTrip`: save a state with `Restored`/`RestoredFiles`, load it back equal; write an old-style JSON `{"base":"x","files":{}}` and load it: `Restored == ""`, `RestoredFiles == nil`.
- [ ] Step 2: run `go test ./internal/local -run TestStateRestored` → FAIL (unknown field).
- [ ] Step 3: add the two fields.
- [ ] Step 4: run → PASS. Commit `feat: record which snapshot a restore put back`.

### Task 2: Status lists restore points

**Files:** Modify `internal/engine/status.go`. Test: `internal/engine/status_test.go`.

**Produces:**
```go
type RestorePoint struct {
	Name    string    // snapshot name
	Time    time.Time // when it was pushed, UTC
	From    string    // display name of the PC that pushed it; its UUID's first 8 chars when unknown
	Current bool      // this PC's files match it
}
// Status.Points []RestorePoint — newest first; nil when unreachable.
```

- [ ] Step 1: tests:
  - `TestStatusPointsNewestFirstAndCurrentBase`: two NJ snapshots, synced to the newer, no local changes → 2 points, newest first, From "NJ", newest current.
  - `TestStatusPointsNoneCurrentWithLocalChanges`: as above plus a local edit → no point current.
  - `TestStatusPointsUnknownPC`: a snapshot from a UUID with no machine record → From is the UUID's first 8 chars.
  - `TestStatusPointsRestored`: state `Restored` = older snapshot, `RestoredFiles` = local hashes → older current, newer not.
  - Update `TestStatus`'s expected value with its `Points`.
- [ ] Step 2: run `go test ./internal/engine -run TestStatus` → FAIL (no Points).
- [ ] Step 3: in `Status`, keep the local hashes; after `s.Reachable = true`, list `d.Snapshots`, read `d.Machines()` once into a uuid→name map, build points newest first; current per the rule (`maps.Equal(st.RestoredFiles, hashes)` when `st.Restored != ""`, else `st.Base` when `LocalChanges == 0`).
- [ ] Step 4: run `go test ./internal/engine` → PASS. Commit `feat: list a flavor's snapshots as restore points in its status`.

### Task 3: RestoreSnapshot

**Files:** Modify `internal/engine/status.go` (next to Restore), `internal/engine/engine.go` (merge clears restored). Test: `internal/engine/restore_test.go` (new).

**Produces:** `var ErrNoRestorePoint = errors.New("no such restore point")`; `func (e *Engine) RestoreSnapshot(f wow.Flavor, name string, opt Options) (RestoreReport, error)` — `RestoreReport.ID` = snapshot name, `Paths` = files rewritten or deleted, `Undo` = backup set.

- [ ] Step 1: tests:
  - `TestRestoreSnapshotPutsFilesBack`: s1 {A:a1,B:b1}, s2 {A:a2}; synced s2; local A=a2 plus unsynced `Config.wtf` → restore s1: A=a1, B=b1, Config.wtf untouched; Paths [A,B]; backup set holds A=a2 (restore of Undo puts a2 back and removes B).
  - `TestRestoreSnapshotDeletesExtraFiles`: restore s2 when local has B → B removed.
  - `TestRestoreSnapshotRecordsRestored`: after restoring s1, state `Restored` = s1, `Base` unchanged; Status shows s1 current and `LocalChanges > 0`.
  - `TestSyncAfterRestoreUploadsIt`: restore s1, then `Push` → new snapshot has a1/b1, state `Restored == ""`.
  - `TestPullClearsRestored`: restore s1, NJ pushes s3, `Pull` (choose remote on conflicts) → `Restored == ""`.
  - `TestRestoreSnapshotUnknownName`: "nope" and a well-formed name not in the folder → `ErrNoRestorePoint`, files unchanged.
  - `TestRestoreSnapshotRefusesWhileRunning` → `ErrGameRunning`.
  - `TestRestoreSnapshotDryRun`: Paths filled, nothing written, no state change.
- [ ] Step 2: run `go test ./internal/engine -run 'RestoreSnapshot|AfterRestore|ClearsRestored'` → FAIL (undefined).
- [ ] Step 3: implement:
```go
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
				return rep, fmt.Errorf("%w (what was there is backup %s: wtfsync restore --flavor %s --set %s)", err, rep.Undo, f.Name, rep.Undo)
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
```
  and in `merge` set `st.Restored, st.RestoredFiles = "", nil` with the new base. (Push already saves a fresh `FlavorState`.)
- [ ] Step 4: run `go test ./internal/engine` → PASS. Commit `feat: restore a flavor to any kept snapshot`.

### Task 4: App — Sync and restore points

**Files:** Modify `internal/app/app.go`, `internal/app/page.go`, `internal/app/view.go`; tests `internal/app/*_test.go`.

**Consumes:** `engine.Status.Points`, `engine.RestoreSnapshot`, `engine.ErrNoRestorePoint`.
**Produces:**
```go
type PointItem struct{ Name, When, From string; Current bool }
// FlavorPage: Points []PointItem, Unreachable bool (Backups removed)
func (a *App) Sync() error                      // every enabled flavor
func (a *App) SyncFlavor(name string) error
func (a *App) RestorePoint(flavor, name string) error
```
Removed: `Push`, `PushFlavor`, `Pull`, `PullFlavor`, `Restore`, `RestorePreview`, `BackupItem`, `backups`, `backupGone`.

- [ ] Step 1: tests (rewrite the Push/Pull/Restore ones in `app_test.go` / `page_test.go` to the new API):
  - Sync pushes local changes and pulls the other PC's; busy text "syncing" / "syncing classic"; missing-addons notice after a sync that finds more.
  - Menu has `sync` "Sync", no push/pull.
  - Page: Points rows with When/From/Current from Status; Unreachable when status unreachable.
  - Badge words "N changes to sync", "Sync pending: cloud folder unreachable"; issue text "sync pending (sync folder unreachable)".
  - RestorePoint restores and records no flavor error; unknown name → error text "That restore point no longer exists".
  - New-settings notice Action "sync".
- [ ] Step 2: `go test ./internal/app` → FAIL.
- [ ] Step 3: implement (Sync = old `push` plus the missing-addons announce from old `pull`; `retryPending` not needed since Push handles pending; RestorePoint uses `perform(..., false, ...)` and wraps `ErrNoRestorePoint` in a `pointGone` error; `friendly` maps `ErrNoRestorePoint`).
- [ ] Step 4: `go test ./internal/app` → PASS. Commit `feat: one Sync action and restore points in the app model`.

### Task 5: Window, tray and demo

**Files:** `internal/ui/bridge_windows.go`, `internal/ui/ui_windows.go`, `internal/ui/web/main.html`, `main.js`, `app.css`, `demo.js`.

- [ ] Bridge: `SyncAll()`, `SyncFlavor(name)`, `RestorePoint(flavor, name)`; drop Push*/Pull*/Restore/RestorePreview.
- [ ] ui: `do("sync")` → `go u.app.Sync()`; notification category `sync` with action "Sync".
- [ ] main.html: top bar `<button id="syncAll">Sync all</button>`.
- [ ] main.js: flavor page Sync button (primary); "Restore points" section: scroll box rows (radio, `When · from From`, "current" tag, current rows not selectable), default pick = first non-current; note text; Restore… → confirm text from the spec → `RestorePoint`; after restore note "Restored. Sync to send this to your other PCs; restore the newest to undo."; empty / unreachable texts.
- [ ] app.css: `.points { max-height: 220px; overflow-y: auto; }`, `.tag` for current.
- [ ] demo.js: Points per flavor (classic_beta: several with an older current after a restore), Badge words "to sync".
- [ ] Verify: `GOOS=windows go vet ./...`; `node --check` the JS; render screenshots via Docker script and look at them. Commit `feat: Sync button and restore points in the main window`.

### Task 6: CLI `sync` and docs

**Files:** `internal/cli/cli.go`, `internal/cli/sync.go`, tests; `README.md`; `docs/screenshots/*`.

- [ ] Step 1: test `TestSyncCommandPushes` in `internal/cli` (sync uploads local changes like push; output uses push's report). README test already checks usage ↔ Command reference.
- [ ] Step 2: run → FAIL (unknown command).
- [ ] Step 3: `case "push", "pull", "sync", "status", "restore"`; in `one`, `"sync"` runs `eng.Push` and prints with op "push"; usage line `sync      download your other PCs' changes, then upload this PC's`; README: Command reference `wtfsync sync`, flag table mentions sync, Use section and Tray section say Sync, restore points described, checklist item for restore points.
- [ ] Step 4: `go test ./...` → PASS. Refresh screenshots. Commit `feat: wtfsync sync command` and `docs: describe Sync and restore points`.
