# Changelog

## [0.11.0](https://github.com/rhwendt/wtfsync/compare/v0.10.2...v0.11.0) (2026-10-22)


### Features

* record which snapshot a restore put back ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([feab48a](https://github.com/rhwendt/wtfsync/commit/feab48a31eecb7958b2d22de731d41971101751b))
* list a flavor's snapshots as restore points in its status ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([dfccc27](https://github.com/rhwendt/wtfsync/commit/dfccc27a8ae4d97a520859a5f924d5bccd8e3b7a))
* restore a flavor to any kept snapshot ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([7a587bf](https://github.com/rhwendt/wtfsync/commit/7a587bff93676ac0f8f277f9f5e2e556ceaa5209))
* one Sync action and restore points in the app model ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([57eeae6](https://github.com/rhwendt/wtfsync/commit/57eeae65a09340cc29e84e13021781088263941e))
* Sync button and restore points in the main window ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([c4d3605](https://github.com/rhwendt/wtfsync/commit/c4d3605557bc36606570f2da56fd2bbdf3baf4ba))
* add a wtfsync sync command ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([9cd60a5](https://github.com/rhwendt/wtfsync/commit/9cd60a5c420f0d2f05630bab81d5b5d6654f2077))

### Bug Fixes

* keep the way back to unsynced edits after a restore, and clear a stale current mark ([#20](https://github.com/rhwendt/wtfsync/issues/20)) ([8300f27](https://github.com/rhwendt/wtfsync/commit/8300f271797522be4f97300c50c9991539ef5764))

## [0.10.2](https://github.com/rhwendt/wtfsync/compare/v0.10.1...v0.10.2) (2026-10-22)


### Bug Fixes

* blank the title bar's icon with a transparent small icon ([#19](https://github.com/rhwendt/wtfsync/issues/19)) ([98feecb](https://github.com/rhwendt/wtfsync/commit/98feecb7ede7b67080b4e6df4064610ee2866632))

## [0.10.1](https://github.com/rhwendt/wtfsync/compare/v0.10.0...v0.10.1) (2026-10-21)


### Bug Fixes

* keep the app icon out of the title bar ([#18](https://github.com/rhwendt/wtfsync/issues/18)) ([04a61aa](https://github.com/rhwendt/wtfsync/commit/04a61aa15b9f844d2b4218e4260781c63c0b1e2d))

## [0.10.0](https://github.com/rhwendt/wtfsync/compare/v0.9.0...v0.10.0) (2026-10-21)


### Features

* make the app icon the top bar's WTF logo, white and reversed ([#16](https://github.com/rhwendt/wtfsync/issues/16)) ([7e7241f](https://github.com/rhwendt/wtfsync/commit/7e7241fff8643cb17f033071b4f0caa2ccee5afc))
* drop the title bar's icon and title; theme the taskbar icon; ⟳ in the top bar ([#16](https://github.com/rhwendt/wtfsync/issues/16)) ([0a5c289](https://github.com/rhwendt/wtfsync/commit/0a5c28909ccda7791d5f46ec32095679521e32dd))

## [0.9.0](https://github.com/rhwendt/wtfsync/compare/v0.8.2...v0.9.0) (2026-10-21)


### Features

* offer an available update in a toast when the window opens ([#17](https://github.com/rhwendt/wtfsync/issues/17)) ([c1428cf](https://github.com/rhwendt/wtfsync/commit/c1428cf7dec0c69c81aa2a3bd79e101696d3774e))

## [0.8.2](https://github.com/rhwendt/wtfsync/compare/v0.8.1...v0.8.2) (2026-10-21)


### Bug Fixes

* embed the WTF icon in the exes so the taskbar and Start menu show it ([#15](https://github.com/rhwendt/wtfsync/issues/15)) ([84b4cdf](https://github.com/rhwendt/wtfsync/commit/84b4cdf9b484781441f638e6a02cd2a2ca51e053))

## [0.8.1](https://github.com/rhwendt/wtfsync/compare/v0.8.0...v0.8.1) (2026-10-21)


### Bug Fixes

* give wtfsync's windows the WTF icon instead of the generic one ([#14](https://github.com/rhwendt/wtfsync/issues/14)) ([8f04b28](https://github.com/rhwendt/wtfsync/commit/8f04b28dd20c3bcccc450ccfc4a9369087504a12))

## [0.8.0](https://github.com/rhwendt/wtfsync/compare/v0.7.0...v0.8.0) (2026-10-21)


### Features

* add an All flavors setting and a per-PC appearance setting ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([b335b81](https://github.com/rhwendt/wtfsync/commit/b335b813052841f668c7c2b8168395569188c67a))
* record why each backup was made and list backups with it ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([1a6d352](https://github.com/rhwendt/wtfsync/commit/1a6d352b94b89d11d1304fc6fda5c48e4faaf4f8))
* know which PC wrote the base snapshot and list a folder's PCs ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([35c8aa5](https://github.com/rhwendt/wtfsync/commit/35c8aa52ca9d87bfbe1e1d7c8a865e1fc402da50))
* describe each flavor for the main window: badge, times, backups, missing addons ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([d4536f5](https://github.com/rhwendt/wtfsync/commit/d4536f5515f96b74d02bb3f818e34e3e8c39cc25))
* per-flavor push, pull and restore, appearance, and an Open wtfsync tray item ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([2c9a742](https://github.com/rhwendt/wtfsync/commit/2c9a7427e3b787d473e02890d0ee0b7f1f40bb0a))
* add the main window's pages (flavor list, flavor page, settings, wizard) ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([f811ef9](https://github.com/rhwendt/wtfsync/commit/f811ef9130b8817f7af814d6cce66dba7afe7bd4))
* open the main window from the tray and drop the old windows ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([fae543d](https://github.com/rhwendt/wtfsync/commit/fae543d1a5381cab184054e737aa7e7ebe155cd2))

### Bug Fixes

* restoring the oldest kept backup no longer deletes it first ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([e6ceb02](https://github.com/rhwendt/wtfsync/commit/e6ceb02924a849b50c61c6d6129184662c3d8de8))
* name the flavor in the conflict dialog ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([adfe5cf](https://github.com/rhwendt/wtfsync/commit/adfe5cfd8d32d3404cf3acf2fb397df874bb543c))
* offer Join in the wizard whenever the cloud folder has wtfsync data ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([7307444](https://github.com/rhwendt/wtfsync/commit/730744457e2d9eaec817dee9a68b4faa86413761))
* plain restore errors, all-or-nothing settings saves, and no stale refreshes ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([2c8a856](https://github.com/rhwendt/wtfsync/commit/2c8a856cd832e98f61f2c8e55833032dda95d96c))
* keep Settings edits on update checks and open the flavor a notification is about ([#13](https://github.com/rhwendt/wtfsync/issues/13)) ([9d5c299](https://github.com/rhwendt/wtfsync/commit/9d5c29965e84ee2191047ebe60c97da3a39c5fd4))

## [0.7.0](https://github.com/rhwendt/wtfsync/compare/v0.6.0...v0.7.0) (2026-10-21)


### Features

* log tray actions, their timing, and every goroutine when one stalls ([#12](https://github.com/rhwendt/wtfsync/issues/12)) ([276c675](https://github.com/rhwendt/wtfsync/commit/276c67557d57f92149bc6877e6903893d8ccb7c6))
* add a health line (goroutines, memory, CPU) for the tray log ([#12](https://github.com/rhwendt/wtfsync/issues/12)) ([45ce71e](https://github.com/rhwendt/wtfsync/commit/45ce71ea19e60d4c3cd5c74d4e36c4cabaec3b52))
* send Wails' warnings, crashes and window activity to the tray's logs ([#12](https://github.com/rhwendt/wtfsync/issues/12)) ([51530e6](https://github.com/rhwendt/wtfsync/commit/51530e62948c0a4afe5447bf4717f668f65ae497))

## [0.6.0](https://github.com/rhwendt/wtfsync/compare/v0.5.1...v0.6.0) (2026-10-21)


### Features

* draw the tray icon as a WTF tile with a coloured state strip ([#11](https://github.com/rhwendt/wtfsync/issues/11)) ([d9f4a45](https://github.com/rhwendt/wtfsync/commit/d9f4a45d2a5aac9221693d83ccc9308f0313158e))

## [0.5.1](https://github.com/rhwendt/wtfsync/compare/v0.5.0...v0.5.1) (2026-10-20)


### Bug Fixes

* give the tray's pages the index.html Wails needs to serve any of them ([#10](https://github.com/rhwendt/wtfsync/issues/10)) ([1224639](https://github.com/rhwendt/wtfsync/commit/122463970cb01227e3e07e08d1535c57bd0bdee7))

## [0.5.0](https://github.com/rhwendt/wtfsync/compare/v0.4.0...v0.5.0) (2026-10-19)


### Features

* add a per-PC lock so only one sync runs at a time ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([a58e06b](https://github.com/rhwendt/wtfsync/commit/a58e06b33660497ad1e2675617ceba698ee43db5))
* let a prompter settle all conflicts in one call ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([f241dee](https://github.com/rhwendt/wtfsync/commit/f241dee49d615161b4d5d5f305cfdb96ab8f69f2))
* refuse CLI syncs while another runs; add wtfsync version ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([046f0f5](https://github.com/rhwendt/wtfsync/commit/046f0f5bf4390ecba61034e0ca4bedaf45b36ac3))
* find newer releases and download a verified installer ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([332ec22](https://github.com/rhwendt/wtfsync/commit/332ec224ace14273dd30079e88fded189e636390))
* add start-at-login, tray icons and a size-capped log file ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([2ad55fd](https://github.com/rhwendt/wtfsync/commit/2ad55fd7cedce89ddf60197f6740aedbe6725771))
* add the tray's view model: icon state, tooltip, menu and window text ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([7280536](https://github.com/rhwendt/wtfsync/commit/7280536e8341a471bdfc8f00aa0da65f49f036d6))
* add tray logic: push/pull/restore, tick, settings and updates ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([821d893](https://github.com/rhwendt/wtfsync/commit/821d893fac7cc25c1cd66103e5f93d3bda2fc2c5))
* add the conflict dialog's prompter ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([77dc4c9](https://github.com/rhwendt/wtfsync/commit/77dc4c935b2be7abe27811c30c29e25d3c54654e))
* add the tray app: icon, menu, windows and notifications ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([9e6797c](https://github.com/rhwendt/wtfsync/commit/9e6797c677ddd8824274e977170cce22c4378ff8))
* build a per-user installer and attach it to releases ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([2819cd8](https://github.com/rhwendt/wtfsync/commit/2819cd867d15b8d44e196aec815ea2a3e28fb94a))

### Bug Fixes

* tie each conflict dialog to its own question so a late close can't cancel the next ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([5133e25](https://github.com/rhwendt/wtfsync/commit/5133e253229a25071f6564f593cf3b044a6c95c4))
* never rewrite PATH from a value NSIS couldn't read in full ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([2427991](https://github.com/rhwendt/wtfsync/commit/2427991c8d8bdafefa625d941796e9035a43b617))
* send empty lists, not null, so tray windows render with nothing found ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([45b8621](https://github.com/rhwendt/wtfsync/commit/45b8621f0489467b2fff40ed89abac9788e3e167))
* let a running sync finish before the tray quits ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([92e13df](https://github.com/rhwendt/wtfsync/commit/92e13dfac91eb5f6d799923c6db9dbbc3548d4d2))
* keep the tray running when toast notifications can't be registered ([#9](https://github.com/rhwendt/wtfsync/issues/9)) ([afe0bff](https://github.com/rhwendt/wtfsync/commit/afe0bff7ba113dec36dcc54e53deae66004e3b80))

## [0.4.0](https://github.com/rhwendt/wtfsync/compare/v0.3.1...v0.4.0) (2026-10-18)


### Features

* let init and link re-run on a PC that's already set up ([#8](https://github.com/rhwendt/wtfsync/issues/8)) ([14554ae](https://github.com/rhwendt/wtfsync/commit/14554aeb55546b448f4e3528d444918e7ae00d04))

## [0.3.1](https://github.com/rhwendt/wtfsync/compare/v0.3.0...v0.3.1) (2026-10-18)


### Bug Fixes

* treat every _<name>_ folder as a flavor, even without WTF ([#6](https://github.com/rhwendt/wtfsync/issues/6)) ([d24b1ac](https://github.com/rhwendt/wtfsync/commit/d24b1acddbf7f16ea645eccd606da279987d9832))

## [0.3.0](https://github.com/rhwendt/wtfsync/compare/v0.2.0...v0.3.0) (2026-10-18)


### Features

* scan every drive for WoW installs during setup ([#5](https://github.com/rhwendt/wtfsync/issues/5)) ([80e9577](https://github.com/rhwendt/wtfsync/commit/80e95779b98d575182d3572b349ceb0b4d5f1781))

### Bug Fixes

* accept a quoted folder path at setup prompts ([#5](https://github.com/rhwendt/wtfsync/issues/5)) ([92ad0eb](https://github.com/rhwendt/wtfsync/commit/92ad0eb481432fb91fed555f6fe5e84a6b660f2f))

## [0.2.0](https://github.com/rhwendt/wtfsync/compare/v0.1.0...v0.2.0) (2026-10-18)


### Features

* list addons other PCs have that this one is missing ([#4](https://github.com/rhwendt/wtfsync/issues/4)) ([f8d5c83](https://github.com/rhwendt/wtfsync/commit/f8d5c838065fb9d099ce750f12d74587411e46c1))

## 0.1.0 (2026-10-18)


### Features

* add module, CI, and WoW flavor discovery ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([a1c9332](https://github.com/rhwendt/wtfsync/commit/a1c9332e222f186f8501934733ccb006f3e3c498))
* detect running game per flavor by executable path ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([ebe5658](https://github.com/rhwendt/wtfsync/commit/ebe565882c97868873dbb96a43df47eaf78c03ba))
* collect SavedVariables files with safe path filter ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([4112e53](https://github.com/rhwendt/wtfsync/commit/4112e53fb9f0171decffeb4ddc6de2114521a842))
* read installed addon list from .toc files ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([c5ebda0](https://github.com/rhwendt/wtfsync/commit/c5ebda03e5ca3e9b03931c6c95410b9b0bbfe9b7))
* add snapshot archive format with integrity checks ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([25cbc42](https://github.com/rhwendt/wtfsync/commit/25cbc428997e14e2026246d61285bc4ba43eff2e))
* add sync folder init/open, machine records, hydration ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([d5425a3](https://github.com/rhwendt/wtfsync/commit/d5425a37b395af1c94026537d02b8403852c8f9d))
* add seq-ordered immutable snapshots with pruning ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([b35414e](https://github.com/rhwendt/wtfsync/commit/b35414ed1d0c2374c1b66d1a11cf54a8ac0fa26f))
* auto-detect cloud-synced folders for setup ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([3c18ca8](https://github.com/rhwendt/wtfsync/commit/3c18ca815c8d8dd36dc5bad2f8ccadbf87139d8d))
* add local settings, sync state, and machine UUID ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([69a5ee6](https://github.com/rhwendt/wtfsync/commit/69a5ee63e9c529d32f5f0cf95a6ca0274da39778))
* back up files before overwrite and restore them ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([3830b84](https://github.com/rhwendt/wtfsync/commit/3830b8486a9c5ab9e613e7942ebb0944d8756fe8))
* add three-way per-file decision ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([8603d94](https://github.com/rhwendt/wtfsync/commit/8603d94b2e9aeb9e569f6ae3228553be39480f8e))
* pull with three-way merge, conflict prompts, and backups ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([2a1101a](https://github.com/rhwendt/wtfsync/commit/2a1101a7e9e413287c20dd2e4cd871df82215d63))
* push snapshots, merging first when behind, with offline retry ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([3aafe11](https://github.com/rhwendt/wtfsync/commit/3aafe117ce52f8eb7feb27b92bfb0ea46400637a))
* add status and restore to the engine ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([74b4f82](https://github.com/rhwendt/wtfsync/commit/74b4f8203858e61f359063dcd0d7a4699fe421c7))
* add CLI with terminal prompts and init/link setup ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([0f97a43](https://github.com/rhwendt/wtfsync/commit/0f97a4388c2000ef5737f34bb81c93e33c4113fb))
* add push/pull/status/restore commands and wtfsync binary ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([a1e9340](https://github.com/rhwendt/wtfsync/commit/a1e9340b911500a4d2eb8a5e196166a79de62b68))

### Bug Fixes

* merge forked snapshots against their common ancestor ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([74568aa](https://github.com/rhwendt/wtfsync/commit/74568aaebd848e144203834c6624c8972eb109f0))
* make restore honor --dry-run and back up what it overwrites ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([4ba9f02](https://github.com/rhwendt/wtfsync/commit/4ba9f025c2515ca7c7962e8be52e99bb5d4fece0))
* refuse to merge paths that differ only in letter case ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([66ffc47](https://github.com/rhwendt/wtfsync/commit/66ffc4733ebc6e87e075a75aff6f3b930e10aade))
* treat a game process with an unreadable path as running ([#2](https://github.com/rhwendt/wtfsync/issues/2)) ([8ee9237](https://github.com/rhwendt/wtfsync/commit/8ee9237cfa9c55df1457d130d8bf03e1dee41da9))
