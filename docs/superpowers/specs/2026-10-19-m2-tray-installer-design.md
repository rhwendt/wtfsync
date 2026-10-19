# wtfsync — Milestone 2 design: tray app, installer, updates

**Status:** approved in brainstorming, awaiting spec review · **Date:** 2026-10-19
**Builds on:** [`2026-10-18-wtfsync-design.md`](2026-10-18-wtfsync-design.md) (the v1 spec). Where
this document differs, this document wins; the v1 spec's "Tray app (M2)" section points here.

## Goal

Make wtfsync something you install once and then mostly ignore: a per-user installer, a tray icon
that shows whether each flavor is in sync, windows for setup, conflicts and settings instead of
terminal prompts, and one-click updates from GitHub Releases. Syncing is still started by the
user in M2 (menu or CLI); automatic push on game exit and the Play button stay in M3.

**Success:** on a fresh PC, run `wtfsync-setup.exe` → the tray appears → the wizard links the PC →
when the other PC pushes, a notification offers a pull → a conflict is settled in a dialog → a new
release installs from the tray menu → uninstall leaves settings and backups in place.

## Decisions

| Topic | Decision |
|---|---|
| UI toolkit | **Wails v3**, pinned to `v3.0.0-beta.32` (v3 is beta; v2 has no tray). Upgrades are deliberate PRs. Checked: tray, menus, attached windows, autostart, single instance and a notifications service exist; a tray + window app cross-compiles from Linux with `CGO_ENABLED=0 GOOS=windows`. |
| Executables | **Two**, sharing all sync code: `wtfsync.exe` (console CLI, unchanged) and `wtfsync-tray.exe` (GUI subsystem, no console). Replaces the v1 spec's single binary with `AttachConsole`, which makes `cmd` return before output and breaks interactive prompts. |
| Front end | Plain HTML/CSS/JS embedded with `go:embed`, no npm or bundler. Pages call Go through the runtime Wails serves at `/wails/runtime.js`. |
| Installer | **NSIS**, per-user (no admin), built on Linux in CI with `makensis`. |
| Updates | Tray checks GitHub Releases at startup and every 24 h; menu offers **Update to vX.Y.Z**; one click downloads, verifies the checksum and runs the installer silently. |
| Automatic sync | None in M2 beyond the existing pending-push retry. Push/pull run when the user asks. |
| Concurrency | One sync at a time per PC, enforced by a lock in the data folder, shared by CLI and tray. |

## Architecture

| Package / dir | Job | Depends on | Tested |
|---|---|---|---|
| `cmd/wtfsync` | CLI entry (unchanged apart from `version`) | `cli` | via `cli` tests |
| `cmd/wtfsync-tray` | Tray entry: real dependencies → `ui.Run` | `ui`, `app` | builds in CI |
| `internal/app` | Tray *logic*, no Wails: per-flavor status, overall icon state, menu model, actions (push/pull/restore/setup/settings), 60 s tick, update check, notification decisions | `engine`, `local`, `syncdir`, `wow`, `update`, `lock` | unit tests on Linux with fakes |
| `internal/ui` | Wails glue: tray icon/menu from `app`'s model, windows, Go services the pages call, toasts | Wails, `app` | builds in CI; manual on Windows |
| `internal/ui/web` | Pages: `wizard.html`, `conflicts.html`, `settings.html`, `status.html`, `addons.html` + one shared `app.css`/`app.js` | — | manual |
| `internal/update` | Latest-release lookup, version compare, download + SHA-256 verify | stdlib | unit tests with `httptest` |
| `internal/lock` | Per-PC sync lock (`LockFileEx` on Windows, `flock` elsewhere) | `x/sys` | unit tests |
| `internal/autostart` | HKCU `…\CurrentVersion\Run` value `wtfsync` (Windows); no-op stub elsewhere | `x/sys/windows/registry` | interface faked in `app` tests |
| `installer/wtfsync.nsi` | NSIS script | — | compiles in CI; manual |

`app` exposes a small API that `ui` renders; `ui` holds no sync logic. Anything that decides
*what* to show or do lives in `app` so it can be tested without Windows.

### Engine changes

- **Batch conflicts.** Optional interface, checked by type assertion in `engine.resolve`:

  ```go
  type BatchPrompter interface {
      // Resolve settles every conflict at once. firstLink is true when this PC has no base yet.
      Resolve(conflicts []string, other string, firstLink bool) (map[string]Choice, error)
  }
  ```

  The tray's prompter implements it (one dialog for all files); the CLI keeps the per-file
  `Prompter`. Returning `ErrAborted`, or a map missing any conflict, writes nothing.
- **Setup is shared.** The `init`/`link` flow (choose folders, re-run rules from v0.4.0, first
  push/pull) moves from `cli` into a package both front ends call (`internal/setup`), with the
  CLI keeping its prompts as one way of supplying the answers.
- **Lock.** `cli` and `app` take `lock.Acquire(dataDir)` around push, pull, restore and setup.
  If it's held: CLI exits 2 with "another wtfsync sync is running"; the tray shows that as a
  notification and does nothing. The lock is released on exit, including crashes (OS file lock).

## Tray behaviour

**Icon states** (overall = most severe flavor):

| State | When | Tooltip example |
|---|---|---|
| idle | every enabled flavor up to date, nothing pending | "wtfsync — in sync" |
| syncing | an action is running | "wtfsync — pulling classic…" |
| new settings | another PC pushed a snapshot newer than this PC's base | "classic: NY has newer settings" |
| attention | unresolved conflict, last action failed, push pending, or missing addons | "classic: push pending (Drive offline)" |

**Menu:** Push · Pull · Status… · Restore last backup… · Missing addons… *(only when any)* ·
Settings… · Update to vX.Y.Z *(only when available)* · Quit. Push/Pull act on all enabled flavors
and are disabled while syncing.

**Notifications** (Windows toasts, one per event, not repeated every tick):
- another PC pushed newer settings → "NY pushed new settings for classic" with a **Pull** action;
- a pull finished and addons are missing → "NY has 2 addons you don't" → opens Missing addons;
- an action failed or was refused → the error text (e.g. "Close WoW (classic) first").

**Tick (60 s):** retry pending pushes, refresh status (no snapshot reads beyond `Newest`), and
raise the "new settings" notification when the newest snapshot changes to another PC's.

**Single instance:** a second launch focuses the running tray; `wtfsync-tray.exe --quit` asks the
running one to exit (used by the installer).

## Windows

- **Setup wizard** — shown at startup when no `settings.json` exists, and from Settings.
  Steps: WoW folder (drive-scan results or browse) → cloud folder (detected list or browse) →
  PC name (default hostname) → "This is my first PC" (init) or "Join my other PC" (link) →
  runs the first push/pull with progress, then shows the result (including missing addons).
- **Conflict dialog** — opened by the `BatchPrompter`. First link: the single choice (use
  <other>'s for all / keep this PC's / choose per file). Otherwise, or after "choose per file":
  one row per file with **keep <other>'s / keep this PC's** and **apply to all**. **Cancel** =
  `ErrAborted` (nothing written). Closing the window = Cancel.
- **Settings** — PC name, WoW folder, cloud folder (saving re-runs setup with the v0.4.0 rules:
  identity kept; moving folders clears sync history), per-flavor on/off, **start at login**.
- **Status** — per flavor: base, newest and who wrote it, local changes, pending push, last error.
- **Missing addons** — the per-PC list from `engine.OtherAddons`, links open in the browser.
- **Restore last backup** — confirm dialog listing the files, then restore (with its undo set).

## Installer (`installer/wtfsync.nsi` → `wtfsync-setup.exe`)

- `RequestExecutionLevel user`; installs to `%LOCALAPPDATA%\Programs\wtfsync\`
  (`wtfsync.exe`, `wtfsync-tray.exe`, `uninstall.exe`).
- Start menu shortcut **wtfsync** → `wtfsync-tray.exe`.
- Options page, both on by default: **Start wtfsync when Windows starts** (HKCU Run value, the
  same one the tray's setting writes) and **Add the wtfsync command to PATH** (user PATH).
- Before copying: `wtfsync-tray.exe --quit`, wait up to 10 s. After install (silent or not):
  start the tray.
- Uninstall entry under HKCU `…\Uninstall\wtfsync` with version, publisher "wtfsync", size.
  Uninstall removes program files, shortcut, Run value and PATH entry; **keeps**
  `%APPDATA%\wtfsync` (settings, state, backups) and the cloud data.
- Unsigned (SmartScreen "More info → Run anyway" on first run). No bundled WebView2.

## Updates (`internal/update`)

- Source: `GET https://api.github.com/repos/rhwendt/wtfsync/releases/latest` (unauthenticated,
  one call at startup and every 24 h). Skipped when the built-in version is `dev`.
- Newer = higher semver than the built-in version, with assets `wtfsync-setup.exe` and
  `checksums.txt`.
- **Update to vX.Y.Z:** download both to a temp folder → verify SHA-256 of the installer against
  `checksums.txt` → wait for any running sync (the lock) → run `wtfsync-setup.exe /S` → quit. A
  checksum mismatch or download error is shown and nothing runs.

## Release pipeline (CI)

- A `version` step runs first: on pushes to `main` it is `next-version.sh` (empty → no release);
  on PRs it is `dev`. Builds use `-ldflags "-X main.version=<v>"` for both executables;
  `wtfsync version` prints it.
- Build job (Linux): `wtfsync.exe`, `wtfsync-tray.exe` (`-H windowsgui`), `makensis` → 
  `wtfsync-setup.exe`, `checksums.txt` (SHA-256 of the three).
- PRs upload them as one artifact; releases attach `wtfsync-setup.exe`, `wtfsync.exe` and
  `checksums.txt`. Release notes as today.

## Errors

| Situation | Behaviour |
|---|---|
| Game running for a flavor | That flavor's action refused; toast "Close WoW (<flavor>) first"; others proceed. |
| Sync already running (lock held) | CLI exit 2; tray toast "A sync is already running". |
| Cloud folder unreachable | Push marked pending (as M1); icon → attention; retried every tick. |
| Conflict dialog cancelled/closed | Nothing written; icon → attention until the next successful pull. |
| WebView2 missing | Wails' own prompt to install it; documented in the README. |
| Update download/checksum fails | Toast with the reason; menu item stays; nothing is run. |
| Tray crashes | OS releases the lock; restarting from the Start menu resumes. Errors go to `%APPDATA%\wtfsync\tray.log` (kept under 1 MB, one previous file). |

## Testing

- **Linux unit tests:** `app` (state per flavor → icon/tooltip/menu; actions call the engine
  with the lock; tick retries pending and raises "new settings" once; notification de-duplication;
  update offered only for newer releases), `update` (`httptest` GitHub API: newer/older/equal,
  missing assets, checksum mismatch, `dev` skips), `lock` (second acquire fails, release frees),
  engine `BatchPrompter` (all resolved / abort / missing entries write nothing), `setup` (moved
  from `cli`, existing tests keep passing).
- **CI:** existing tests on ubuntu + windows; `GOOS=windows go vet ./...`; build both executables;
  `makensis` compiles the installer.
- **Manual checklist (real PCs):** install → tray + Start menu entry; wizard on a fresh PC
  (link); push on one PC → toast on the other → Pull; same-file conflict → dialog; game running
  → refusal toast; CLI and tray at once → "already running"; Settings rename + start at login;
  update vN → vN+1 from the menu; uninstall keeps `%APPDATA%\wtfsync`.

## To verify during planning

- Wails v3 beta.32 on Windows: toast notifications from an unsigned per-user app (AppUserModelID
  / Start menu shortcut requirements), tray icon swapping at runtime, `Call.ByName` naming for
  bound services, attached-window vs. normal-window behaviour for the dialogs.
- NSIS on `ubuntu-latest`: `makensis` version and the plugins needed (none beyond stock).

## Out of scope for M2

- Automatic push/pull (M3), Play button, Battle.net launch.
- Code signing, winget, auto-update without a click.
- macOS/Linux tray.
