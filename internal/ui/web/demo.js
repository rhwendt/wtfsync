// Sample data for screenshots: the same shapes ui.Bridge returns. Used only with ?demo.
const p = new URLSearchParams(location.search);

const main = {
  SetUp: true, Busy: "", PC: "desktop", Version: "v0.8.0", Update: p.has("update") ? "v0.10.0" : "", Appearance: p.get("theme") || "system",
  Flavors: [
    { Name: "classic", Badge: { Kind: "ok", Short: "✓", Words: "In sync" },
      Line: "Last synced Oct 19, 02:07 · newest from laptop", Unreachable: false,
      Points: [
        { Name: "p3", When: "Oct 19, 02:07", From: "laptop", Current: true },
        { Name: "p2", When: "Oct 18, 21:40", From: "desktop", Current: false },
      ], Missing: [] },
    { Name: "classic_beta", Badge: { Kind: "changes", Short: "↑ 26", Words: "26 changes to sync" },
      Line: "Last synced today 03:51 · newest from laptop", Unreachable: false,
      Points: [
        { Name: "b8", When: "today 03:51", From: "laptop", Current: false },
        { Name: "b7", When: "yesterday 22:15", From: "desktop", Current: false },
        { Name: "b6", When: "Oct 20, 23:02", From: "laptop", Current: false },
        { Name: "b5", When: "Oct 19, 02:07", From: "desktop", Current: false },
        { Name: "b4", When: "Oct 17, 21:40", From: "desktop", Current: true },
        { Name: "b3", When: "Oct 16, 20:12", From: "laptop", Current: false },
        { Name: "b2", When: "Oct 15, 19:30", From: "desktop", Current: false },
        { Name: "b1", When: "Oct 14, 22:48", From: "laptop", Current: false },
      ],
      Missing: [{ PC: "laptop", Addons: [
        { Name: "Details!", URL: "https://www.curseforge.com/projects/61284", Folders: "Details" },
        { Name: "Plater", URL: "https://www.curseforge.com/projects/100547", Folders: "Plater" },
      ] }] },
    { Name: "classic_era", Badge: { Kind: "new", Short: "new", Words: "Not synced yet" }, Line: "Never synced", Unreachable: false, Points: [], Missing: [] },
    { Name: "retail", Badge: { Kind: "newer", Short: "↓", Words: "laptop has newer settings" },
      Line: "Last synced Oct 18, 21:10 · laptop synced today 08:12", Unreachable: false,
      Points: [
        { Name: "r2", When: "today 08:12", From: "laptop", Current: false },
        { Name: "r1", When: "Oct 18, 21:10", From: "desktop", Current: true },
      ], Missing: [] },
  ],
};

const answers = {
  Main: () => main,
  Settings: () => ({ Name: "desktop", WoWDir: "C:\\Program Files (x86)\\World of Warcraft", SyncFolder: "G:\\My Drive\\wtfsync",
    AllFlavors: false, StartAtLogin: true,
    Flavors: [{ Name: "classic", On: true }, { Name: "classic_beta", On: true }, { Name: "classic_era", On: false }, { Name: "retail", On: true }] }),
  Defaults: () => ({ Installs: ["C:\\Program Files (x86)\\World of Warcraft"], Clouds: ["G:\\My Drive\\wtfsync"],
    WoWDir: "C:\\Program Files (x86)\\World of Warcraft", Sync: "G:\\My Drive\\wtfsync", Name: "desktop", Rerun: false }),
  CloudFolder: () => ({ HasData: true, PCs: ["laptop"] }),
  Conflicts: () => ({ ID: 1, Flavor: "classic", Other: "laptop", FirstLink: false, Files: [
    "Account/WOWPLAYER/SavedVariables/ElvUI.lua",
    "Account/WOWPLAYER/SavedVariables/Details.lua",
    "Account/WOWPLAYER/Area 52/Thrall/SavedVariables/WeakAuras.lua",
  ] }),
  RestorePoint: () => null,
  SaveSettings: () => ["Settings saved; sync history kept."],
  Setup: () => ["classic: got laptop's settings (12 file(s))"],
};

export async function call(method) {
  return answers[method] ? answers[method]() : null;
}
