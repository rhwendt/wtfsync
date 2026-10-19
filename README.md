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

Grab `wtfsync.exe` from the latest [release](https://github.com/rhwendt/wtfsync/releases/latest).

Releases are automatic: every merge to `main` that includes a `feat:`, `fix:`, `perf:` or breaking
change ([Conventional Commits](https://www.conventionalcommits.org/)) publishes a new version. `fix`
bumps the patch number, `feat` the minor, and a breaking change (`feat!:` or a `BREAKING CHANGE:`
footer) the major (the minor while still 0.x). Merges with only `docs:`, `chore:`, `ci:`, `test:` or
`refactor:` commits don't release.

Each release's notes are the changelog: its features, fixes and breaking changes, grouped from the
commit messages, with the PR each came from. See [all releases](https://github.com/rhwendt/wtfsync/releases).

## Build

    GOOS=windows GOARCH=amd64 go build -o wtfsync.exe ./cmd/wtfsync

CI builds `wtfsync.exe` for every PR (artifact `wtfsync-windows-amd64`).

## Manual test checklist (real PCs)

1. `wtfsync init --dry-run`: the flavors and file list look right.
2. `push` / `pull` with `--wow-dir` pointed at a *copy* of the install.
3. NJ ↔ NY through Google Drive for Desktop (including streaming/placeholder mode).
4. Change the same addon on both PCs without syncing: the second push asks.
