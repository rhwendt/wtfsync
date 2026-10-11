# WoW config sync

**Status:** design agreed (brainstormed in vault, promoted 2026-10-11) · **Created:** 2026-10-11

Sync WoW addon config (SavedVariables, not system config) between PCs, so either machine picks up where the
other left off.

## Who / why
- Me first: one PC in NJ, one in NY.
- Probably a real (small, edge-case) need for others — keep that door open, don't build for it yet.

## What actually needs syncing
- `WTF/Account/<ACCT>/SavedVariables/*.lua` — account-wide addon settings
- `WTF/Account/<ACCT>/<Realm>/<Char>/SavedVariables/*.lua` — per-character addon settings
- `WTF/Config.wtf` — **excluded (decided)**: system/graphics CVars are per-machine (NJ and NY
  PCs have different hardware/settings). Maybe later: an opt-in allowlist of non-hardware CVars.
- Keybinds/macros/Blizzard UI: Blizzard already server-syncs these
- Addon *code* (`Interface/AddOns`): don't store — store a manifest instead

## Addon manifest (instead of storing addon files)
- `.toc` files often carry `## X-Curse-Project-ID`, `## X-Wago-ID`, `## X-WoWI-ID` — free, no API key.
- CurseForge fingerprint-match API identifies the rest, but needs an approved API key + ToS.
- Not everything is on CurseForge (Wago, WoWInterface, GitHub) — manifest = folder + version + source id(s).

## Hard part
- WoW writes SavedVariables only on logout/`/reload`, and **overwrites from memory on exit** —
  syncing while the game runs gets silently clobbered. Need game-running detection + conflict handling.

## Prior art (checked 2026-10-11)
- [WowSync](https://github.com/Rmkrs/WowSync) — local Windows tool, merges SavedVariables across
  *accounts on one machine*; dry-run, backups, undo. No cross-machine/cloud. Worth reading its code.
- CurseForge app "AddonSync" — syncs the addon *list* between machines (needs login), not settings.
- WowUp — no cloud backup.
- Folk method: junction/symlink `WTF` into a Dropbox folder — works, but races with the game and
  syncs hardware-specific config.

## Sync model (decided: C)
Tray app that auto-pushes when WoW exits and auto-pulls before it launches, **plus** manual
Push/Pull buttons as override. Build order: manual push/pull first, then automation.

## Storage: bring-your-own-cloud via a synced folder
No cloud APIs/OAuth. User picks a folder that their cloud client already syncs (Google Drive for
Desktop, iCloud for Windows, OneDrive, Dropbox); the app writes snapshots there and the client moves them.

Setup flow:
- **First PC:** detect WoW install → pick sync folder (offer auto-detected Drive/OneDrive/iCloud/Dropbox
  paths) → app creates its own subfolder there.
- **Next PC:** pick the same folder → app sees existing data → linked.

Gotchas to design for:
- Cloud-client lag: the other PC may not have the newest snapshot yet when you pull.
- Files-on-demand / placeholders (Drive streaming, iCloud, OneDrive) — file may not be local yet.
- Cloud "conflict copies" (`file (1)`) if two machines write the same file → write **immutable,
  uniquely named snapshots** (timestamp + machine id) instead of overwriting one file.

## Launching WoW through the app
"Play" button in the app: check freshness → pull → launch WoW (via Battle.net). Process watching
still catches exit (→ push) and WoW started some other way (→ warn "not synced").
- **Session marker:** on launch, write "NJ playing since <time>" to the sync folder; clear it with
  the snapshot on exit. If NY sees an open NJ marker with no newer snapshot → warn: NJ still playing,
  or its upload hasn't arrived yet. Handles cloud lag.

## Encryption (decided: yes)
- Shared passphrase, set on first PC, entered once per new PC.
- `age` passphrase mode (scrypt KDF); passphrase cached in Windows Credential Manager so you
  aren't asked again.
- Lost passphrase = cloud snapshots unreadable; local `WTF` is untouched, so recover by re-pushing.
- Maybe later: pairing code / QR instead of typing the passphrase.

## Conflicts (decided: C)
- Compare **per SavedVariables file** against the last-synced state; newest wins per file, so
  different addons changed on different PCs both survive.
- Same file changed on both PCs → ask ("ElvUI changed on both — keep NJ or NY?").
- Always back up local files before a pull overwrites them (restorable).
- No merging *inside* a file — not worth it for one player on two PCs.

## Stack & distribution (decided: Go)
- Go, single `.exe`; UI via Wails or Fyne (tray + setup wizard + conflict prompt).
- Encryption via `age` passphrase mode (scrypt KDF); passphrase cached in Windows Credential Manager.
- GitHub Releases first; code signing (Azure Trusted Signing ~$10/mo) + winget only if others use it.

## Components
| Package | Job |
|---|---|
| `wow` | Find install, flavors, `WTF/Account/*`; watch game processes; launch via Battle.net |
| `collect` | Gather SavedVariables (skip `Config.wtf`, `*.lua.bak`), hash files, read `.toc` → addon list |
| `vault` | Archive + manifest, `age` encrypt/decrypt, Credential Manager |
| `syncdir` | Cloud folder: immutable snapshots, session markers, force placeholder hydration |
| `engine` | Push/pull, per-file compare vs last-synced hashes, conflict decisions, backups |
| `ui` | Tray (Play/Push/Pull/status), setup wizard, conflict prompt |

Cloud layout: `wowsync/<flavor>/snapshots/<time>_<machine>.age`, `wowsync/<flavor>/sessions/<machine>.json`,
`wowsync/vault.json`. Local: `%APPDATA%\wowsync\` (settings, last-synced hashes, backups).

**Flavors (decided): all by default.** Every detected `_<flavor>_` folder (retail, classic, classic
era, PTR…) is synced independently — own snapshots, own markers, own process
(`Wow.exe`, `WowClassic.exe`, `WowT.exe`…). Per-flavor toggle in settings. Verify exact process
names + Battle.net launch codes per flavor.

**Addon list (decided): informational only in v1** — "NJ has X and Y, not installed here". No
auto-install (would need CurseForge API key).

## Data flow (approved)
**Play** (tray/app button), per flavor:
1. Read newest snapshot + other machines' session markers from the sync folder.
2. Other machine has an open marker with no newer snapshot → warn "NJ is still playing or hasn't
   finished uploading — Play anyway / Wait".
3. Pull (below) → write own session marker → launch via Battle.net.

**Push** (on game exit, or manual):
1. Wait a few seconds after process exit so WoW finishes writing files.
2. Collect + hash. Nothing changed since last sync → just clear the marker.
3. Otherwise encrypt → write new immutable snapshot → clear marker → save last-synced hashes.

**Pull:**
1. Hydrate + decrypt newest snapshot.
2. Per file, three-way compare (last-synced hash vs local vs remote): remote-only change → take;
   local-only → keep; both → ask.
3. Back up every local file about to be overwritten → write → update last-synced hashes.
4. Show addon-list differences.
5. **Refuse while that flavor's game process is running.**

**Errors:**
- Sync folder missing/offline → queue snapshot locally, retry later. Never lose an exit push.
- Wrong passphrase / corrupt snapshot → skip it and report; never touch local files.
- Tray "Restore previous" → restore last local backup.
- Retention: keep newest N snapshots per flavor (default 20), prune older.

## Testing
- `engine`: table tests for the three-way decision (take/keep/ask/no-op) with in-memory files.
- `collect`: fixture `WTF/` trees (multiple accounts, chars, flavors, `.bak`, `Config.wtf` excluded).
- `vault`: encrypt/decrypt round-trip; wrong passphrase fails cleanly.
- `syncdir`: unique naming, retention pruning, markers, against a temp dir.
- Manual end-to-end: NJ ↔ NY via a real Google Drive folder, including "play on both" conflict.

## Out of scope for v1
- Syncing `Config.wtf` / any system CVars.
- Installing or updating addons.
- Merging inside a single SavedVariables file.
- Cloud APIs/OAuth, own backend, mobile, macOS.
