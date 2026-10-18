# wtfsync — v1 design

**Status:** approved in brainstorming, awaiting spec review · **Date:** 2026-10-18
**Source:** [`docs/brainstorm.md`](../../brainstorm.md) (decisions there stand unless this spec says otherwise)

## Goal

Sync World of Warcraft addon settings (SavedVariables) between Windows PCs through a folder the
user's own cloud client already syncs (Google Drive for Desktop, OneDrive, iCloud, Dropbox), so
either PC picks up where the other left off.

- **Primary user:** one player, two PCs (NJ and NY). Design supports N machines; don't build
  features for other users yet.
- **Success:** play on NJ, exit, launch on NY → NY has NJ's addon settings, with no silent
  clobbering when both PCs changed something.

## Decisions (and changes from the brainstorm)

| Topic | Decision |
|---|---|
| Name | `wtfsync` everywhere: binary, cloud folder, `%APPDATA%` folder. (Brainstorm used `wowsync`; that name collides with the existing WowSync tool.) |
| Spec scope | All of v1 in one spec, delivered in three milestones. |
| Stack | Go, single `wtfsync.exe`. UI via **Wails** (WebView2, no CGO → cross-compiles from Linux). Pin the Wails version (v3 if its tray support is stable) during planning. |
| Machine identity | Random **UUID** generated at setup; used in every filename. **Display name** is a separate, user-editable label (defaults to hostname) and never affects identity. |
| Encryption | **Removed from v1** (brainstorm had it as decided). Data is low-sensitivity and lives in the user's own cloud account; cost in UX and code isn't worth it. Format leaves room to add it — see [Later](#later). |
| Deletions | A deleted file is a change like any other and goes through the three-way compare. |
| Snapshot ordering | By a per-flavor sequence number, not wall-clock time. |
| Push | Always merges the newest remote snapshot first if this PC is behind. |
| Conflicts, flavors, addon list, Config.wtf | As in the brainstorm: per-file three-way compare, ask on conflict; all flavors by default; addon list informational only; `Config.wtf` excluded. |

## Milestones

1. **Core CLI.** `init`, `link`, `push`, `pull`, `status`, `restore`; terminal conflict prompts;
   offline retry; retention. Fully testable on Linux with fake trees; testable on Windows with
   `--dry-run`.
2. **Tray app.** Wails tray menu (Push / Pull / Status / Restore), setup wizard, conflict dialog,
   settings (start at login, per-flavor toggles, display name).
3. **Automation.** Process watching with auto-push on exit, Play button with session markers and
   Battle.net launch, warning when WoW was started outside the app.

## Architecture

One binary. `wtfsync.exe` with no arguments starts the tray app (M2+); with a subcommand it runs
the CLI. It is built as a GUI-subsystem executable (no console window for the tray); in CLI mode
it attaches to the parent console (`AttachConsole(ATTACH_PARENT_PROCESS)`) so output prints.

| Package | Job | Depends on | Windows-only parts (behind interfaces, faked in tests) |
|---|---|---|---|
| `wow` | Find install root and flavors; list/watch game processes; launch via Battle.net | — | Install detection via Battle.net config, process listing, launching |
| `collect` | Walk a flavor's `WTF/`, select SavedVariables, hash them; read `.toc` files → addon list | — | none |
| `archive` | Build/read snapshot `.tar.gz` (manifest + files) | — | none |
| `syncdir` | Sync-folder layout: list/order/write snapshots, session markers, machine records, placeholder hydration, retention pruning | `archive` | none |
| `engine` | Push/pull orchestration, three-way compare, conflict resolution via a `Prompter`, backups, pending-push retry, local state | `collect`, `archive`, `syncdir`, `wow` | none |
| `cli` | Subcommands, flags, terminal `Prompter` | `engine` | console attach (in `main`) |
| `ui` | Wails tray, wizard, conflict dialog; thin front end over Go bindings | `engine` | Wails, Run-key autostart |

Key interfaces (names indicative; finalise in the plan):

- `wow.Processes` — `Running(flavor) (bool, error)`; M3 adds a watch loop on top.
- `wow.Launcher` — `Launch(flavor) error`.
- `engine.Prompter` — `ResolveConflict(file, localDesc, remoteDesc) (Choice, error)` and
  `FirstLink(diffCount) (FirstLinkChoice, error)`. Terminal, Wails, and scripted (test) implementations.
- `syncdir.Hydrator` — forces a cloud placeholder to download (default: open and read the whole
  file with a timeout). Faked in tests.

## Data layout

### Sync folder

```
<user-chosen folder>/wtfsync/
  wtfsync.json                                   # {"format": 1, "encryption": "none", "created": "<utc>"}
  machines/<uuid>.json                           # {"uuid", "name", "lastSeen"}
  <flavor>/snapshots/<seq>_<utc>_<uuid>.tar.gz   # immutable, never rewritten
  <flavor>/sessions/<uuid>.json                  # open session marker (M3)
```

- `<flavor>` is the install subfolder name without underscores: `retail`, `classic`,
  `classic_era`, `ptr`, … (whatever `_<name>_` folders contain a `WTF/`).
- `<seq>` is zero-padded to 6 digits; `<utc>` is `YYYYMMDDTHHMMSSZ`.
- Files are only ever created, never overwritten (avoids cloud-client conflict copies). Markers
  are the exception: a machine only ever writes its own marker file, so no two machines write the
  same path.
- Anything else in the folder (e.g. `file (1).tar.gz` conflict copies, temp files) is ignored.

**Ordering.** A new snapshot's `seq` = highest `seq` this machine can see in that flavor + 1.
Newest = highest `seq`; ties broken by `<utc>`, then by UUID. Clock skew therefore can't make an
old snapshot look new; two machines pushing the same `seq` while offline still get unique names
(different UUIDs), and the tie-break is deterministic.

**Writing.** Write to `<name>.tmp` in the same directory, then rename, so the cloud client never
uploads a half-written snapshot.

### Snapshot contents

A gzip-compressed tar containing `manifest.json` first, then the files at their `WTF/`-relative paths.

```json
{
  "format": 1,
  "machine": { "uuid": "3f9a1c2e-…", "name": "NJ" },
  "created": "2026-10-18T15:30:00Z",
  "flavor": "retail",
  "seq": 42,
  "base": "000041_20261017T220000Z_7b2d…",
  "files": { "Account/123#1/SavedVariables/ElvUI.lua": "sha256:…" },
  "addons": [ { "folder": "ElvUI", "version": "13.74", "curse": "…", "wago": "…", "wowi": "…" } ]
}
```

- `base` = the snapshot this machine was synced with when it pushed (empty for the first push).
- Every snapshot is a **full** copy of the flavor's SavedVariables, so a pull only needs the newest one.

### What is collected (per flavor)

- `WTF/Account/*/SavedVariables/*.lua` (account-wide)
- `WTF/Account/*/*/*/SavedVariables/*.lua` (per character: `<Account>/<Realm>/<Char>/`)
- Excluded: `*.lua.bak`, `WTF/Config.wtf`, and everything else under `WTF/`.
- Addon list: each `Interface/AddOns/*/*.toc` → folder, `## Version`, and any of
  `## X-Curse-Project-ID`, `## X-Wago-ID`, `## X-WoWI-ID`.

### Local data, `%APPDATA%\wtfsync\`

- `settings.json` — machine UUID, display name, sync folder, WoW install root, per-flavor enabled flags.
- `state/<flavor>.json` — last-synced snapshot name + per-file hashes (the **base** for the three-way
  compare), and a `pendingPush` flag set when a push couldn't reach the sync folder.
- `backups/<flavor>/<utc>/` — files as they were before a pull overwrote or deleted them, plus an
  index of which were absent. Keep the newest 10 sets per flavor.

## Setup

`wtfsync init` (first PC) and `wtfsync link` (next PCs); the M2 wizard runs the same steps.

1. **Find WoW.** Default install path, then Battle.net's config, else ask. List detected flavors
   (all enabled by default).
2. **Pick the sync folder.** Offer auto-detected Google Drive / OneDrive / iCloud / Dropbox paths,
   or any folder. `init` creates `wtfsync/` and `wtfsync.json`; `link` requires them to exist.
   `init` on a folder that already has `wtfsync/` stops and suggests `link`.
3. **Name this PC.** Generate a UUID; display name defaults to the hostname. Write `machines/<uuid>.json`.
4. **First pull** (`link`) or **first push** (`init`).

## Data flow

### Three-way compare

Per file path, with base = last-synced hash, local = this PC, remote = newest snapshot.
"Absent" is a value, so deletions follow the same rules.

| Case | Action |
|---|---|
| local = remote | no-op; record as synced |
| local = base, remote ≠ base | **take remote**: back up, then write (or delete if remote is absent) |
| remote = base, local ≠ base | **keep local** |
| local ≠ base, remote ≠ base, local ≠ remote (incl. edit vs. delete) | **conflict**: ask "ElvUI.lua changed on NJ and here — keep NJ / keep this PC" |

**First link** (no base for this flavor): rather than one conflict per differing file, ask once —
*use the other PC's settings for everything (recommended) / keep this PC's / ask per file*.

### Pull

Refused while that flavor's game process is running ("Close WoW Classic first").

1. Find newest snapshot. If it is the base → "up to date", stop.
2. Hydrate (timeout 60s → "Newest snapshot hasn't downloaded yet", stop) and read it.
3. Three-way compare; resolve conflicts via the `Prompter`.
4. Back up every local file about to be overwritten or deleted.
5. Apply; save new base = this snapshot's name and file hashes.
6. Print addon differences ("NJ has WeakAuras, not installed here").

### Push

Also refused while that flavor's game process is running.

1. If newest remote snapshot ≠ base → run **Pull** steps 2–5 (merge), so other machines' changes
   aren't hidden by this push.
2. Collect + hash. Identical to the base → nothing to push, stop (M3: clear marker).
3. Build snapshot with `seq` = max visible + 1 and `base` = current base.
4. Write to the sync folder (tmp + rename). If the folder is unreachable, set `pendingPush` and
   stop — the local files are still on disk, so nothing is lost; the retry re-runs Push from step 1
   and builds a fresh snapshot (so its `seq` and merge are correct at that time).
5. Save new base, clear `pendingPush`; prune that flavor to the newest 20 snapshots (never prune
   the snapshot that is any machine's current open-marker base).
6. M3: clear this machine's session marker.

### `status` and `restore`

- `status` per flavor: base snapshot, newest remote (and who pushed it), count of local changes
  since base, pending push, open session markers from other machines.
- `restore [--flavor F] [--set <utc>]`: puts back the most recent (or the given) backup set,
  including re-deleting files that were absent; lists available sets with no `--set`. Refused
  while the game is running.

### CLI surface (M1)

```
wtfsync init | link | push | pull | status | restore
  --flavor <name>     limit to one flavor (default: all enabled)
  --dry-run           print the plan (take/keep/conflict/delete per file); change nothing
  --wow-dir <path>    override detected install root
  --sync-dir <path>   override configured sync folder
  --data-dir <path>   override %APPDATA%\wtfsync (lets one machine simulate two PCs)
```

Exit codes: `0` ok, `1` error, `2` refused (game running / not hydrated), `3` unresolved conflict
(user aborted the prompt).

## Tray app (M2)

- Menu: **Push**, **Pull**, **Status**, **Restore previous**, **Settings**, **Quit**; icon shows
  idle / syncing / needs attention.
- Setup wizard mirrors `init`/`link`; shown when no `settings.json` exists.
- Conflict dialog: one row per conflicting file with *keep <other PC> / keep this PC*, plus
  "apply to all".
- Settings: start at login (HKCU `…\Run` key), per-flavor toggles, display name.
- Tray tick every 60s: retry any pending push, refresh status.

## Automation (M3)

- **Watching:** poll the process list every 3s for each enabled flavor's executable. On exit, wait
  5s for WoW to finish writing, then **Push**. If the push hits a conflict, the tray shows "needs
  attention" and the push waits until the dialog is resolved.
- **Session marker** `sessions/<uuid>.json`: `{"uuid", "name", "started", "base"}`. Written on
  Play (or when a launch is detected), cleared after the exit push.
- **Play**, per flavor:
  1. Read other machines' markers. If one is open and that machine has no snapshot newer than the
     marker's `base` → warn "NJ is still playing or hasn't finished uploading — Play anyway / Wait".
     Markers older than 24h are labelled "possibly stale (crash?)".
  2. Pull.
  3. Write own marker.
  4. Launch: `Battle.net.exe --exec="launch <code>"`.
- **Started outside the app:** tray notice "WoW started without syncing — changes will be merged
  when you exit". Safe because Push always merges first; worst case is conflict prompts.
- **To verify on the real PC** (don't trust from memory): executable name and Battle.net launch
  code for each flavor; Battle.net install path and config location.

## Errors

| Situation | Behaviour |
|---|---|
| Sync folder missing/unreachable on push | Set `pendingPush`; retried on the next CLI run / tray tick. An exit push is never lost (the data is the local files). |
| Newest snapshot is a placeholder that won't hydrate in 60s | Report; change nothing. |
| Corrupt/truncated snapshot or unknown `format` is the newest | Stop pull/push for that flavor and report which file and which machine wrote it; never touch local files. The writing machine's next push supersedes it. |
| Game running | Refuse pull/push/restore for that flavor (exit code 2). |
| Prompt aborted | Change nothing for that flavor (exit code 3). |
| `wtfsync.json` missing on `link` | "No wtfsync data in this folder — run `init` on your first PC." |

## Testing

- **`engine`**: table tests covering every base/local/remote combination including absent
  (deletion) values; first link (all three choices); push-when-behind merges first. In-memory
  files, scripted `Prompter`.
- **`collect`**: fixture `WTF/` trees — multiple accounts, realms, characters, flavors; `.bak`
  files and `Config.wtf` excluded; `.toc` parsing with and without source IDs.
- **`archive`**: round-trip; corrupt and truncated archives fail with a clear error.
- **`syncdir`** (temp dir): `seq` ordering incl. skewed clocks; same-`seq` tie-break; tmp+rename;
  ignoring conflict copies; retention (incl. not pruning a marker's base); markers;
  hydration timeout via fake `Hydrator`.
- **`wow`**: path and flavor parsing (portable); processes and launcher behind interfaces.
- **Two-PC scenarios** (drive `cli` with separate `--wow-dir`/`--data-dir` for `nj/` and `ny/`
  plus one shared `--sync-dir`): push→pull; different addons edited on each (both survive); same
  addon edited on both (conflict); deletion propagates; sync folder offline then back (pending push retried); restore.
- **CI (GitHub Actions):** `go vet` + `go test ./...` on `ubuntu-latest` and `windows-latest`;
  `//go:build windows` tests (real process listing, Run key) run on the Windows runner; build and
  upload `wtfsync.exe` as an artifact on every PR.
- **UI:** Go binding logic unit-tested; screens checked manually.
- **Manual checklist on the real PCs:**
  1. `wtfsync init --dry-run` against the real install — flavors and file list look right.
  2. `push` / `pull` with `--wow-dir` pointed at a *copy* of the install.
  3. NJ ↔ NY through Google Drive for Desktop (incl. streaming/placeholder mode).
  4. Play on both PCs without syncing → conflict prompt on the second exit.
  5. M3: confirm process names and launch codes per flavor.

## Out of scope for v1

- Encryption (see Later).
- Syncing `Config.wtf` or any CVars.
- Installing or updating addons.
- Merging inside a single SavedVariables file.
- Cloud APIs/OAuth, own backend, mobile, macOS.
- Code signing and winget (GitHub Releases only).

## Later

- **Encryption (opt-in):** `init` generates an `age` X25519 key; `wtfsync.json` stores it wrapped
  with a passphrase (scrypt) and sets `"encryption": "age"`; snapshots become `.tar.gz.age`;
  the key is cached in Windows Credential Manager. Scrypt then runs once per PC, passphrase
  changes only re-wrap the key, and a pairing code/QR can transfer the key.
- Opt-in allowlist of non-hardware CVars from `Config.wtf`.
- Auto-install missing addons (needs a CurseForge API key).
- Code signing (Azure Trusted Signing) + winget if others use it.
