// The main window: flavor list, the selected flavor's page, Settings, and the setup wizard.
import { $, h, call, on, close, applyTheme, errorText, plural, openURL, params, folderPicker } from "./app.js";
import { wizard } from "./wizard.js";

const KEY = "wtfsync.flavor";
const remembered = () => { try { return localStorage.getItem(KEY) || ""; } catch { return ""; } };
const remember = (f) => { try { localStorage.setItem(KEY, f); } catch { /* not kept: fine */ } };
// The update the toast's Later was clicked for: it stays hidden until a newer one is on offer.
const LATER = "wtfsync.updateLater";
const later = () => { try { return localStorage.getItem(LATER) || ""; } catch { return ""; } };

let data = null;                              // MainPage from Go
let view = params.get("view") || "flavor";    // "flavor", "settings" or "wizard"
let flavor = params.get("flavor") || remembered();
let settingsShown = false;                    // the settings form is built (rebuilding would lose edits)
let wizardShown = false;
const picked = {};                            // flavor → chosen backup id
let confirming = null;                        // {flavor, id, paths} while a restore is being confirmed
let note = null;                              // {text, error} shown above the page
let installing = false;                       // Update now was clicked; until the next load

const show = (...children) => $("#page").replaceChildren(...children);
const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

async function load() {
  try {
    data = await call("Main");
  } catch (e) {
    note = { text: errorText(e), error: true };
    return drawNote();
  }
  applyTheme(data.Appearance);
  installing = false; // from here data.Busy says whether the update is still going
  if (!data.SetUp && view !== "wizard") view = "wizard";
  render();
}

// run does an action, shows its error (if any) above the page, then reloads.
async function run(fn) {
  note = null;
  try {
    await fn();
  } catch (e) {
    note = { text: errorText(e), error: true };
  }
  await load();
}

function drawNote() {
  $("#note").textContent = note ? note.text : "";
  $("#note").className = note && note.error ? "error" : "";
}

function render() {
  const busy = data.Busy !== "";
  $("#busy").textContent = busy ? cap(data.Busy) + "…" : "";
  $("#pullAll").disabled = $("#pushAll").disabled = busy || !data.SetUp;
  document.body.classList.toggle("wizard-mode", view === "wizard");
  drawNote();
  drawToast();
  if (view === "wizard") return startWizard();
  wizardShown = false;
  renderList();
  if (view === "settings") return settingsPage();
  settingsShown = false;
  flavorPage();
}

// update starts installing the offered version. If it can't start (a sync is running), the next
// load shows the toast as it was; a failed download is reported as a notification.
function update() {
  installing = true;
  drawToast();
  call("InstallUpdate");
  setTimeout(load, 1500);
}

// drawToast offers an update in the corner while one is waiting, unless Later was clicked for it.
// The window and tray restart once it's installed.
function drawToast() {
  const v = data.Update;
  const t = $("#toast");
  const busy = installing || data.Busy === "updating";
  t.hidden = !v || view === "wizard" || (later() === v && !busy);
  if (t.hidden) return;
  t.replaceChildren(
    h("b", {}, "Update available"),
    h("div", { class: "muted" }, busy ? `Installing ${v}… wtfsync restarts when it's done.` : `wtfsync ${v} is ready to install.`),
    h("div", { class: "row" },
      h("button", { class: "primary", disabled: busy, onclick: update }, "Update now"),
      h("button", { disabled: busy, onclick: () => { try { localStorage.setItem(LATER, v); } catch { /* shown again next time */ } t.hidden = true; } }, "Later")));
}

function go(to) {
  view = to;
  note = null;
  confirming = null;
  render();
}

function renderList() {
  const fl = data.Flavors;
  if (!fl.some((f) => f.Name === flavor)) flavor = fl.length ? fl[0].Name : "";
  $("#list").replaceChildren(
    ...fl.map((f) => h("button", {
      class: "item" + (f.Name === flavor && view === "flavor" ? " on" : ""),
      onclick: () => { flavor = f.Name; remember(flavor); go("flavor"); },
    }, h("span", { class: "name" }, f.Name), h("span", { class: "badge " + f.Badge.Kind }, f.Badge.Short))),
    h("div", { class: "me" }, "This PC: ", h("b", {}, data.PC), h("br"), data.Version));
}

function flavorPage() {
  const f = data.Flavors.find((x) => x.Name === flavor);
  if (!f) return show(h("p", { class: "muted" }, "No flavors are being synced. Turn some on in Settings (⚙)."));
  const busy = data.Busy !== "";
  show(
    h("div", { class: "head" }, h("h1", {}, f.Name), h("span", { class: "grow" }),
      h("button", { disabled: busy, onclick: () => run(() => call("PullFlavor", f.Name)) }, "Pull"),
      h("button", { class: "primary", disabled: busy, onclick: () => run(() => call("PushFlavor", f.Name)) }, "Push")),
    h("div", {}, h("span", { class: "badge wide " + f.Badge.Kind }, f.Badge.Words)),
    h("p", { class: "muted" }, f.Line),
    h("h2", {}, "Backups"), backups(f, busy),
    h("h2", {}, "Missing addons"), missing(f));
}

function backups(f, busy) {
  if (confirming && confirming.flavor === f.Name) return confirmRestore(f);
  if (f.Backups.length === 0) return h("p", { class: "muted" }, "No backups yet. One is made each time a pull or restore changes files here.");
  if (!f.Backups.some((b) => b.ID === picked[f.Name])) picked[f.Name] = f.Backups[0].ID;
  return h("div", {},
    h("div", { class: "backups" }, f.Backups.map((b) =>
      h("label", { class: "backup" + (b.ID === picked[f.Name] ? " on" : "") },
        h("input", { type: "radio", name: "backup", checked: b.ID === picked[f.Name], onchange: () => { picked[f.Name] = b.ID; render(); } }),
        h("span", { class: "grow" }, b.When, b.Reason ? " · " + b.Reason : ""),
        h("span", { class: "muted" }, plural(b.Files, "file"))))),
    h("div", { class: "row" },
      h("span", { class: "muted grow" }, "Puts those files back as they were then. What's there now is backed up first."),
      h("button", { disabled: busy, onclick: () => run(async () => {
        const id = picked[f.Name];
        confirming = { flavor: f.Name, id, paths: await call("RestorePreview", f.Name, id) };
      }) }, "Restore…")));
}

function confirmRestore(f) {
  const c = confirming;
  const b = f.Backups.find((x) => x.ID === c.id);
  return h("div", { class: "confirm" },
    h("p", {}, `Restore ${plural(c.paths.length, "file")} as they were ${b ? b.When : "then"}?`),
    h("ul", { class: "files" }, c.paths.map((p) => h("li", {}, h("code", {}, p)))),
    h("div", { class: "row end" },
      h("button", { onclick: () => { confirming = null; render(); } }, "Cancel"),
      h("button", { class: "primary", disabled: data.Busy !== "", onclick: () => run(async () => {
        confirming = null;
        picked[f.Name] = await call("Restore", f.Name, c.id);
        note = { text: "Restored. What was there before is the backup now selected, so you can put it back." };
      }) }, "Restore")));
}

function missing(f) {
  if (f.Missing.length === 0) return h("p", { class: "muted" }, "None");
  return h("div", {}, f.Missing.map((g) => h("p", {}, `${g.PC} has: `,
    g.Addons.map((a, i) => [i ? " · " : "",
      a.URL ? h("a", { href: "#", title: a.Folders, onclick: (e) => { e.preventDefault(); openURL(a.URL); } }, a.Name)
            : h("span", { title: a.Folders }, a.Name)]))));
}

async function settingsPage() {
  if (settingsShown) return;
  settingsShown = true;
  let s;
  try {
    s = await call("Settings");
  } catch (e) {
    s = null;
    if (view === "settings") show(h("p", { class: "error" }, errorText(e)));
  }
  if (view !== "settings" || !s) { settingsShown = false; return; } // left Settings while it loaded
  const name = h("input", { type: "text", value: s.Name });
  const wow = folderPicker([], s.WoWDir, "Your World of Warcraft folder");
  const cloud = folderPicker([], s.SyncFolder, "A folder your cloud client syncs");
  const all = h("input", { type: "checkbox", class: "switch", checked: s.AllFlavors });
  let toggles = [];
  const each = h("div", { class: "flavors", hidden: s.AllFlavors });
  const drawToggles = (list) => {
    toggles = list.map((t) => ({ name: t.Name, box: h("input", { type: "checkbox", class: "switch", checked: t.On }) }));
    each.replaceChildren(...toggles.map((t) => h("label", { class: "inline" }, t.box, t.name)));
  };
  drawToggles(s.Flavors);
  // A different WoW folder has its own flavors: keep the choices for names it shares, new ones off.
  wow.select.addEventListener("change", async () => {
    const was = Object.fromEntries(toggles.map((t) => [t.name, t.box.checked]));
    const names = await call("FlavorNames", wow.value());
    drawToggles(names.map((n) => ({ Name: n, On: !!was[n] })));
  });
  all.addEventListener("change", () => { each.hidden = all.checked; });
  const login = h("input", { type: "checkbox", class: "switch", checked: s.StartAtLogin });
  const theme = h("div", { class: "seg" }, [["system", "System"], ["light", "Light"], ["dark", "Dark"]].map(([v, label]) =>
    h("label", {}, h("input", { type: "radio", name: "theme", checked: data.Appearance === v, onchange: () => {
      applyTheme(v);
      call("SetAppearance", v).catch((e) => { note = { text: errorText(e), error: true }; drawNote(); });
    } }), label)));
  const field = (label, ...el) => h("div", { class: "field" }, h("label", {}, label), ...el);
  const aboutText = () => (data.Update ? `wtfsync ${data.Version} · ${data.Update} available` : `wtfsync ${data.Version} · up to date`);
  const about = h("span", { class: "grow" }, aboutText());
  const updateBtn = h("span", {});
  const drawUpdate = () => {
    about.textContent = aboutText();
    updateBtn.replaceChildren(data.Update ? h("button", { class: "primary", onclick: update }, "Update to " + data.Update) : "");
  };
  drawUpdate();
  const checkUpdates = async () => {
    note = null;
    try {
      await call("CheckUpdate");
      data = await call("Main"); // only the About row changes: unsaved edits stay
      drawUpdate();
    } catch (e) {
      note = { text: "Couldn't check for updates: " + errorText(e), error: true };
    }
    drawNote();
  };
  const save = h("button", { class: "primary", onclick: () => run(async () => {
    const lines = await call("SaveSettings", {
      Name: name.value, WoWDir: wow.value(), SyncFolder: cloud.value(), AllFlavors: all.checked,
      Flavors: toggles.map((t) => ({ Name: t.name, On: t.box.checked })), StartAtLogin: login.checked,
    });
    note = { text: lines.join("\n") || "Saved." };
    settingsShown = false; // show what was saved
  }) }, "Save");
  show(
    h("a", { href: "#", class: "back", onclick: (e) => { e.preventDefault(); go("flavor"); } }, "← Back"),
    h("h1", {}, "Settings"),
    h("h2", {}, "This PC"), field("Name", name),
    h("h2", {}, "Folders"), field("WoW folder", wow.el), field("Cloud folder", cloud.el),
    h("h2", {}, "Flavors"), h("label", { class: "inline" }, all, "All flavors, including ones installed later"), each,
    h("h2", {}, "Appearance"), theme,
    h("h2", {}, "Startup"), h("label", { class: "inline" }, login, "Start wtfsync when Windows starts"),
    h("h2", {}, "About"),
    h("div", { class: "row" }, about,
      h("button", { onclick: checkUpdates }, "Check for updates"), updateBtn,
      h("button", { onclick: () => call("OpenLogFolder") }, "Open log folder")),
    h("div", { class: "row end" },
      h("button", { onclick: () => go("wizard") }, "Run setup again…"), h("span", { class: "grow" }),
      h("button", { onclick: () => go("flavor") }, "Cancel"), save));
}

function startWizard() {
  if (wizardShown) return;
  wizardShown = true;
  const from = data.SetUp ? (settingsShown ? "settings" : "flavor") : null;
  settingsShown = false;
  wizard($("#page"), {
    current: () => view === "wizard",
    start: Math.max(0, Number(params.get("step") || 1) - 1),
    done: () => { view = "flavor"; load(); },
    // On a PC that isn't set up, Cancel just hides the window; the tray still offers Set up.
    cancel: from ? () => { view = from; load(); } : () => close(),
  });
}

$("#pullAll").onclick = () => run(() => call("PullAll"));
$("#pushAll").onclick = () => run(() => call("PushAll"));
$("#gear").onclick = () => go("settings");

let pending = null; // coalesce bursts of "changed" (a sync emits several)
on("changed", () => { clearTimeout(pending); pending = setTimeout(load, 150); });
on("show", (v) => {
  if (v && v.startsWith("flavor:")) {
    if (v.length > 7) { flavor = v.slice(7); remember(flavor); }
    go("flavor");
  } else if (v) go(v);
  else load();
});
await load();
