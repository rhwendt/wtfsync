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

Its menu has Push, Pull, Status, Restore last backup, Missing addons, Settings, Update and Open
log folder. A
conflict (the same addon changed on both PCs) opens a window to choose which version to keep;
Cancel changes nothing. The tray and the `wtfsync` command never sync at the same time: whichever
starts second says "a sync is already running".

**Something wrong?** The menu's **Open log folder** opens `%APPDATA%\wtfsync`, which holds:

- `tray.log`: each menu click, window, sync (with how long it took and any error), a health line
  every 10 minutes (memory, CPU, open windows), and Wails' own warnings. A sync still running
  after a minute also logs every goroutine's stack, which shows where a stuck tray is stuck.
- `crash.log`: the full stack trace if the tray crashes.

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
