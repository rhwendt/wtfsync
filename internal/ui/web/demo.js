// Sample data for screenshots: the same shapes ui.Bridge returns. Used only with ?demo.
const p = new URLSearchParams(location.search);

const main = {
  SetUp: true, Busy: "", PC: "beehive", Version: "v0.8.0", Update: "", Appearance: p.get("theme") || "system",
  Flavors: [
    { Name: "classic", Badge: { Kind: "ok", Short: "✓", Words: "In sync" },
      Line: "Last synced Oct 19, 02:07 · newest from leviathan",
      Backups: [{ ID: "b3", When: "Oct 19, 02:07", Reason: "before pull from leviathan", Files: 4 }], Missing: [] },
    { Name: "classic_beta", Badge: { Kind: "changes", Short: "↑ 26", Words: "26 changes to push" },
      Line: "Last synced today 03:51 · newest from leviathan",
      Backups: [
        { ID: "b2", When: "today 03:51", Reason: "before pull from leviathan", Files: 12 },
        { ID: "b1", When: "Oct 19, 02:07", Reason: "before pull from leviathan", Files: 4 },
        { ID: "b0", When: "Oct 18, 22:40", Reason: "before restore", Files: 4 },
      ],
      Missing: [{ PC: "leviathan", Addons: [
        { Name: "Details!", URL: "https://www.curseforge.com/projects/61284", Folders: "Details" },
        { Name: "Plater", URL: "https://www.curseforge.com/projects/100547", Folders: "Plater" },
      ] }] },
    { Name: "classic_era", Badge: { Kind: "new", Short: "new", Words: "Not synced yet" }, Line: "Never synced", Backups: [], Missing: [] },
    { Name: "retail", Badge: { Kind: "newer", Short: "↓", Words: "leviathan has newer settings" },
      Line: "Last synced Oct 18, 21:10 · leviathan pushed today 08:12", Backups: [], Missing: [] },
  ],
};

const answers = {
  Main: () => main,
  Settings: () => ({ Name: "beehive", WoWDir: "C:\\Program Files (x86)\\World of Warcraft", SyncFolder: "G:\\My Drive\\wtfsync",
    AllFlavors: false, StartAtLogin: true,
    Flavors: [{ Name: "classic", On: true }, { Name: "classic_beta", On: true }, { Name: "classic_era", On: false }, { Name: "retail", On: true }] }),
  Defaults: () => ({ Installs: ["C:\\Program Files (x86)\\World of Warcraft"], Clouds: ["G:\\My Drive\\wtfsync"],
    WoWDir: "C:\\Program Files (x86)\\World of Warcraft", Sync: "G:\\My Drive\\wtfsync", Name: "beehive", Rerun: false }),
  CloudPCs: () => ["leviathan"],
  Conflicts: () => ({ ID: 1, Other: "leviathan", FirstLink: false, Files: [
    "Account/WOWPLAYER/SavedVariables/ElvUI.lua",
    "Account/WOWPLAYER/SavedVariables/Details.lua",
    "Account/WOWPLAYER/Area 52/Thrall/SavedVariables/WeakAuras.lua",
  ] }),
  RestorePreview: () => ["Account/WOWPLAYER/SavedVariables/ElvUI.lua", "Account/WOWPLAYER/SavedVariables/Details.lua"],
  Restore: () => "b9",
  SaveSettings: () => ["Settings saved; sync history kept."],
  Setup: () => ["classic: got leviathan's settings (12 file(s))"],
};

export async function call(method) {
  return answers[method] ? answers[method]() : null;
}
