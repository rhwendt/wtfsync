# wtfsync — main window design

**Status:** approved in brainstorming, awaiting spec review · **Date:** 2026-10-22
**Builds on:** [`2026-10-19-m2-tray-installer-design.md`](2026-10-19-m2-tray-installer-design.md)
(M2). Where this document differs, this document wins: it replaces M2's "Windows" section. M2's
tray icon, installer, updates, lock and release pipeline are unchanged.

## Goal

M2 shipped the tray with one bare window per menu item (Status, Settings, Restore, Missing addons,
the wizard). They work but read like raw data: snapshot file names instead of times,
"0 local change(s)", "in sync" for flavors that never synced, buttons cut off. Replace them with
**one designed main window** where you pick a WoW flavor and see and do everything about it. The
tray stays as the always-running entry point.

**Success:** clicking the tray icon opens a window that shows at a glance whether each flavor is in
sync; from it you can push, pull, restore any of the kept backups, see missing addons, change
settings (including light/dark), and run setup — without the tray menu.

## Decisions

| Topic | Decision |
|---|---|
| Layout | **Flavor first** (mockup C): flavor list on the left, the selected flavor's page on the right, Pull all / Push all / ⚙ in the top bar. Chosen over a sidebar of sections and over tabs. |
| What's in the window | Everything: status, push/pull, backups and restore, missing addons, settings, setup wizard. Only the conflict dialog stays a separate window. |
| Opening | Left-click on the tray icon, or **Open wtfsync** in the tray menu. Closing hides it; the tray keeps running. At login only the tray starts, unless the PC isn't set up (then the window opens on the wizard). |
| Backups | Listed per flavor, newest first; any kept set can be restored (not just the newest). |
| Appearance | Settings → Appearance: **System** (default) / Light / Dark, per PC, applied immediately. |
| Toolkit | Stays Wails v3 (pinned `v3.0.0-beta.32`) with plain HTML/CSS/JS embedded via `go:embed`, no build step. |
| Addon sync | **Out of scope**; its own design next (copy only missing addons, "Copy from <PC>"). The Missing addons section leaves room for that button. |

## The window

**Frame.** Title "wtfsync", about 860×560 at first, resizable (minimum about 640×420); every page
reflows or scrolls rather than clipping. A single instance: opening it again focuses it.

**Top bar** (dark, as the tray icon): WTF logo · "wtfsync" · while an action runs, what it is
("Pulling classic…") · **Pull all** · **Push all** · **⚙**. Pull all / Push all and every per-flavor
action are disabled while an action runs.

**Flavor list** (left): one row per enabled flavor with a badge:

| Badge | Colour | When (most important first) |
|---|---|---|
| `!` | red | last action failed, push pending, cloud folder unreachable |
| `↓` | amber | another PC pushed settings newer than this PC's last sync |
| `↑ N` | amber | N files changed here since the last sync |
| `new` | grey | never synced |
| `✓` | green | none of the above |

Missing addons don't change the badge; they show on the flavor page. Under the list: "This PC:
<name>" and the version. The last selected flavor is remembered (browser storage is enough).

**Flavor page** (right):
- Name, badge with words ("26 changes to push", "leviathan has newer settings", "Not synced yet",
  "In sync", the error), **Pull** and **Push** for this flavor only.
- Status line: "Last synced today 03:51 · newest from leviathan" / "Last synced Oct 19, 02:07 ·
  leviathan pushed Oct 20, 03:51" / "Never synced". Times are local: "today HH:MM", "yesterday
  HH:MM", otherwise "Mon D, HH:MM" (with the year when it isn't this year). Snapshot file names
  never appear.
- The last action's error, if any, in red.
- **Backups:** a pick-one list, newest first: date, reason ("before pull from leviathan",
  "before restore", or nothing for older backups that didn't record one) and file count. Below it
  the note "Puts those files back as they were then. What's there now is backed up first." and
  **Restore…**, which asks for confirmation listing the set's files, then restores and selects the
  new "before restore" set so it can be undone. No backups → "No backups yet. One is made each
  time a pull or restore changes files here."
- **Missing addons:** grouped by PC ("leviathan has: Details! · Plater"), names linking to their
  download page when known (https only, opened in the browser). None → "None".

**Settings** (⚙) replaces the flavor page, with "← Back" (selecting a flavor also leaves it):
- This PC: name.
- Folders: WoW folder, cloud folder, each with Browse….
- Flavors: on/off switch per flavor found in the WoW folder.
- Appearance: System / Light / Dark — applies and saves immediately.
- Startup: Start wtfsync when Windows starts.
- About: version and update state ("up to date" / "v0.9.0 available"), **Check for updates**,
  **Update to vX** (when available), **Open log folder**.
- **Run setup again…** · Cancel · **Save**. Save follows the existing rules (identity kept;
  moving to another sync folder clears sync history; a folder with wtfsync data is joined).

**Setup wizard** fills the window (no flavor list), with a step bar: WoW folder → Cloud folder →
PC name → First PC or join → Sync. Steps and rules as M2 (drive scan / detected folders /
Browse…, hostname default, init or link, first sync with progress, then the result including
missing addons). Step 4 names the PC(s) already in the cloud folder when there are any ("Join
leviathan"). Finish → main view. Cancel → back to Settings when opened from there; on a PC that
isn't set up, it closes the window (the tray still offers Set up).

**Conflict dialog** stays its own window, opened on top (it can arrive while the main window is
hidden), restyled to match: "<flavor> · these files changed on <PC> and on this PC since you last
synced", one row per file with a **<PC>'s | this PC's** toggle, "Set all to <PC>'s / this PC's",
**Cancel** · **Apply**, and "Cancel changes nothing. Files you replace are backed up first." The
first-link question (use <PC>'s for all / keep this PC's / choose per file) comes first, as now.
Closing it = Cancel. One window per question, as now.

**Look.** One stylesheet for every page: Segoe UI, the badge colours above (same as the tray
icon), blue primary buttons, light and dark palettes from the approved mockups. The theme follows
the Appearance setting (System = Windows' setting, updated live when Windows switches). The window
background matches the theme before the page loads.

## Tray

Icon unchanged (WTF tile with state strip). Left-click opens the window. Menu:
**Open wtfsync** · Push · Pull · Settings… (opens the window on Settings) · Update to vX *(when
available)* · Open log folder · Quit. Not set up: **Set up wtfsync…** · Open log folder · Quit.
The Status…, Restore last backup… and Missing addons… items and their windows are removed.

## Architecture

The M2 split holds: `internal/app` decides what to show and does it (testable on Linux);
`internal/ui` renders and binds. New and changed pieces:

| Where | Change |
|---|---|
| `internal/app` | `FlavorPage` per flavor: name, badge (kind + short text + words), status line, error, backups, missing addons. Time wording (`when(t, now)`). Badge rules above. `Backups(flavor)` and `Restore(flavor, id)` for a chosen set (`""` = newest, as now). `SetAppearance`. Menu model per the Tray section. Replaces `StatusRows`/`AddonRows`. |
| `internal/engine` | Records why a backup was made: pull from <PC>, restore. Status gains the time and PC of the base snapshot (for the status line). |
| `internal/local` | Backup index gains `reason` (`"pull"` + `from`, or `"restore"`); old indexes without it read as no reason. `BackupInfo{ID, Time, Reason, From, Files}` listing. Settings gain `appearance` (`"system"`, `"light"`, `"dark"`; empty = system). |
| `internal/ui` | One main window (named `main`) instead of five; tray left-click and menu wiring; Bridge methods for the page data, actions, backups, restore, settings, appearance; a Wails event `changed` emitted from the existing `Changed` hook so the page reloads its data. Window background from the theme. |
| `internal/ui/web` | `main.html` (flavor list, flavor page, settings, wizard as views of one page), `conflicts.html`, shared `app.css` / `app.js`, `index.html` kept. `status.html`, `restore.html`, `addons.html`, `settings.html`, `wizard.html` removed. |

The CLI is unchanged (`restore --set` already restores any set); its `status` output may reuse the
new time wording but doesn't have to.

## Errors

| Situation | Behaviour |
|---|---|
| Action fails | Error on the flavor page and as a notification (as M2); badge `!`. |
| Cloud folder unreachable | Badge `!` "Cloud folder unreachable"; Push stays enabled (it marks the push pending, retried every tick, as M2). |
| Restore of a set that was pruned meanwhile | Error "That backup no longer exists"; the list reloads. |
| A page fails to load | The themed background plus Wails' error; `tray.log` has the cause (M2 diagnostics). |
| Appearance value unknown | Treated as System. |

## Testing

- **Linux unit tests** (`internal/app`, `internal/local`, `internal/engine`): badge and words for
  each state and their priority (error > pending > unreachable > newer > local changes > never
  synced > in sync); status line wording; time wording (today, yesterday, this year, other year);
  backup listing newest first with reason and from, including old indexes without a reason;
  restoring a chosen set (and the new undo set); a pruned set errors; appearance saved, unknown
  read as system; tray menu items set up and not set up.
- **CI:** unchanged — tests on ubuntu and windows, `GOOS=windows go vet`, build both executables
  and the installer.
- **Manual (real PCs), added to the README checklist:** tray click opens and focuses the window,
  close hides it; badges match reality on two PCs; restore an older backup, then undo it; Light /
  Dark / System, including Windows switching theme while open; wizard on a fresh PC; conflict
  dialog while the window is hidden.

## Out of scope

- Addon sync (next design).
- Changing how many backups or snapshots are kept (10 / 20, fixed).
- Showing file-level diffs inside a backup or conflict.
- Leaving Wails; macOS/Linux UI.
