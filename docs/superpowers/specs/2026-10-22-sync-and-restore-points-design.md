# wtfsync — one Sync button and restore points

**Status:** approved in brainstorming, awaiting spec review · **Date:** 2026-10-22
**Builds on:** [`2026-10-22-main-window-design.md`](2026-10-22-main-window-design.md). Where this
document differs, this document wins: it replaces that design's Push / Pull buttons and its
Backups section.

## Goal

Two things feel wrong in the main window today:

- **The PC that pushed has nothing to restore.** Backups are only made on a PC when a pull or a
  restore changes its files, so the PC you sync *from* always shows "No backups yet". Every PC
  should be able to go back to how a flavor was at any recent sync ("last Friday"), the same way.
- **Push and Pull are two buttons for one job.** Push already merges what another PC pushed before
  it uploads, so "pull, then push" is what one Push does. The app is called wtfsync; the button
  should be **Sync**.

**Success:** on either PC, a flavor page lists the recent syncs from the cloud folder, newest first,
with the one the files match now marked; picking an older one and clicking Restore puts it back,
and restoring the newest undoes that. One Sync button (and Sync all) replaces Push and Pull in the
window and the tray.

## Decisions

| Topic | Decision |
|---|---|
| Restore source | The cloud folder's kept snapshots (every push), not this PC's local backups. |
| How many | All kept: up to 20 per flavor (unchanged), newest first, in a list that scrolls on its own. |
| The current one | The snapshot this PC's files match is marked **current** and can't be restored; the others can. Usually that's the newest; after a restore it's the one restored. |
| Local backups | Still made before anything replaces local files, still 10 kept, but no longer shown in the window. The CLI `restore` still reaches them (rare case: you took the other PC's version in a conflict and want yours back). |
| Push / Pull | Replaced by **Sync** (= today's Push) in the window and the tray. The CLI keeps `push` and `pull` and gains `sync`. |
| Auto-sync | **Out of scope**; its own design next (sync when WoW closes and when the tray starts). |

## Sync

- **What it does:** exactly today's `Engine.Push`: if another PC pushed since this PC last synced,
  merge that first (conflict window when both changed a file), then upload a new snapshot if
  anything changed here, otherwise upload nothing. A pending push (cloud folder unreachable) works
  as now.
- **Window:** the flavor page has one **Sync** button; the top bar has **Sync all** (every enabled
  flavor) instead of Pull all / Push all. Disabled while an action runs, as now.
- **Tray menu:** **Open wtfsync** · **Sync** · Settings… · Update to vX *(when available)* · Open
  log folder · Quit. Push and Pull are removed.
- **Words:** what the window and notifications say uses "sync", not "push": badge `↑ N` reads "N
  changes to sync"; a running action reads "Syncing classic…"; a pending push reads "Sync pending
  (cloud folder unreachable)". The `↓` badge ("leviathan has newer settings") is unchanged.
- **CLI:** `wtfsync sync` does what `wtfsync push` does (same flags, same exit codes). `push` and
  `pull` stay for scripts. The README's Command reference gains `sync`.

## Restore points

**The list** replaces the flavor page's Backups section, under the heading **Restore points**:

- One row per snapshot in the cloud folder for this flavor, newest first: the time (same wording as
  the status line: "today 21:40", "yesterday 08:15", "Fri Oct 17, 21:40") and "from <PC>" (the
  machine record's name; the PC's short id when there's no record).
- Rows don't show a file count: that would mean downloading every snapshot, and with Google Drive
  streaming they may not be on this PC yet.
- The row this PC's files match is marked **current** (see below) and has no Restore button. The
  list has a fixed height and scrolls on its own; the rest of the page doesn't move.
- Below it: "Puts this flavor's settings back as they were then. What's there now is saved
  first." and **Restore…** for the selected row.
- **Restore…** asks: "Restore classic to Fri Oct 17, 21:40 from laptop? Your current files are
  saved first. Sync afterwards to send this to your other PCs." · Cancel · **Restore**.
- No snapshots → "Nothing synced yet. A restore point is added each time you sync changes."
- Cloud folder unreachable → "Can't reach the cloud folder, so restore points can't be listed."

**Which row is current.** Worked out from local state, never by opening snapshots:

- The flavor state gains `restored` (snapshot name) and `restoredFiles` (that snapshot's file
  hashes), set by a restore and cleared whenever a sync or pull records a new `base`.
- If `restored` is set and this PC's files hash to `restoredFiles`: that row is current.
- Otherwise, if there are no local changes since `base`: the `base` row is current.
- Otherwise no row is current (the files have changes not yet synced); every row can be restored.

**What a restore does** (`Engine.RestoreSnapshot(flavor, name)`):

1. Refuses while the flavor's WoW is running (as every action does).
2. Reads the snapshot (hydrating it as pull does; "not downloaded yet" fails the same way).
3. Makes the synced files in the WTF folder equal the snapshot: writes every file in it, and
   deletes synced files that aren't in it. Files wtfsync doesn't sync are never touched.
4. Before writing, backs up the files it will overwrite or delete (local backup, reason
   "restore"), so the CLI can undo it even if the snapshot is pruned later.
5. Saves `restored` / `restoredFiles`. `base` and `files` are unchanged, so the page then shows "N
   changes to sync": the next Sync uploads the restored settings as a new snapshot (the newest, for
   every PC). If another PC synced meanwhile, that Sync merges as usual and asks about files both
   changed.

**Undo:** restore the row that was current before (normally the newest). No extra button.

## Architecture

| Where | Change |
|---|---|
| `internal/syncdir` | Unchanged API; `Snapshots`, `Machine`, `ReadSnapshot` cover listing and reading. |
| `internal/local` | `FlavorState` gains `Restored string` and `RestoredFiles map[string]string` (`json:"restored,omitempty"`, `"restoredFiles,omitempty"`). Old state files read as not restored. Backups unchanged. |
| `internal/engine` | `RestorePoints(f) ([]RestorePoint, error)`: `RestorePoint{Name, Time, From, Current}`, newest first; `syncdir.ErrUnreachable` when the folder can't be reached. `RestoreSnapshot(f, name, opt)`: the steps above. `Push` and `Pull` clear `Restored*` when they record a new base. The existing `Restore` (local backups) stays for the CLI. |
| `internal/app` | `FlavorPage` carries `RestorePoints` instead of `Backups` (plus an unreachable flag). `RestorePoint(flavor, name)` runs `RestoreSnapshot` as an action. `Sync()` / `SyncFlavor(name)` replace `Push`/`Pull`/`PushFlavor`/`PullFlavor` for the UI. Tray menu per the Sync section. Wording changes above. |
| `internal/ui` | Bridge methods renamed to match; tray menu items `sync` instead of `push` / `pull`. |
| `internal/ui/web` | Flavor page: Sync button, Restore points list (scrolling box, current marker, confirm). Top bar: Sync all. `demo.js` gets restore points (one current, one after a restore) so the screenshots show them. |
| `internal/cli` | `sync` command (alias of `push`). Usage text and README Command reference updated together (the existing test checks they match). |

## Errors

| Situation | Behaviour |
|---|---|
| Cloud folder unreachable | List shows the unreachable note; Sync marks the push pending as now. |
| Snapshot pruned between listing and restoring | "That restore point no longer exists"; the list reloads. |
| Snapshot not downloaded yet | Same error and wording as a pull of a snapshot that isn't downloaded. |
| WoW running | Same refusal as every action. |
| Restore fails part-way | The backup taken in step 4 holds what was there; the error says the CLI `restore` can put it back. |

## Testing

- **Linux unit tests:**
  - `RestorePoints`: newest first; PC names from machine records, short id without one; current =
    `base` when no local changes, = `restored` after a restore, none when files changed since;
    unreachable folder.
  - `RestoreSnapshot`: files equal the snapshot afterwards (written, extra synced files deleted,
    unsynced files untouched); a local backup of what it replaced; `restored*` saved; `base`
    unchanged so status shows the changes to sync; a pruned name errors; refuses while WoW runs.
  - A sync after a restore uploads a snapshot with the restored files and clears `restored*`.
  - Old state files without `restored*` read fine.
  - App: Sync / Sync all run push; tray menu items; badge and status wording.
  - CLI: `sync` runs push; usage and README stay in step.
- **CI:** unchanged.
- **Manual (real PCs), README checklist:** on the PC that pushed, the list shows its own pushes
  with the newest current; restore an older one, the page shows changes to sync and that row
  current; sync, then the other PC's Sync picks it up; restore the newest to undo.

## Out of scope

- Auto-sync (next design).
- Pinning a restore point so it's never pruned.
- Changing how many snapshots (20) or backups (10) are kept.
- Showing the local backups in the window.
- Showing which files differ between restore points.
