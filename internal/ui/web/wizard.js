// The setup wizard, drawn into the main window's page area.
import { h, call, errorText, folderPicker } from "./app.js";

const STEPS = ["WoW folder", "Cloud folder", "PC name", "First PC or join", "Sync"];

// wizard runs setup inside root. done() runs after Finish; cancel() on Cancel.
export async function wizard(root, { start = 0, done, cancel }) {
  const d = await call("Defaults");
  const wow = folderPicker(d.Installs, d.WoWDir, "Your World of Warcraft folder");
  const cloud = folderPicker(d.Clouds, d.Sync, "A folder your cloud client syncs");
  const name = h("input", { type: "text", value: d.Name });
  let hasData = false;   // the chosen cloud folder already has wtfsync data
  let pcs = [];          // the other PCs in it whose records could be read (may be empty)
  let first = true;      // "This is my first PC"
  let step = Math.min(start, 3);
  let error = "";
  let result = null;     // lines from Setup, on the last step

  const lookUp = async () => {
    const c = await call("CloudFolder", cloud.value());
    hasData = c.HasData;
    pcs = c.PCs;
    first = !hasData;
  };
  if (step >= 3) await lookUp();

  async function next() {
    error = "";
    if (step === 1) await lookUp();
    if (step === 2 && !name.value.trim()) { error = "This PC needs a name."; return draw(); }
    if (step === 3) return sync();
    step++;
    draw();
  }

  async function sync() {
    step = 4;
    result = null;
    draw();
    try {
      result = await call("Setup", { WoWDir: wow.value(), SyncFolder: cloud.value(), Name: name.value.trim(), First: first });
    } catch (e) {
      error = errorText(e);
      step = 3;
    }
    draw();
  }

  const choice = (value, title, sub) => h("label", { class: "choice" },
    h("input", { type: "radio", name: "mode", checked: first === value, onchange: () => { first = value; } }),
    h("div", {}, h("b", {}, title), h("div", { class: "muted" }, sub)));

  function body() {
    switch (step) {
      case 0: return [h("h1", {}, "Where is World of Warcraft installed?"),
        h("p", { class: "muted" }, "The folder that holds _retail_, _classic_ and the other flavors."), wow.el];
      case 1: return [h("h1", {}, "Which cloud folder should wtfsync use?"),
        h("p", { class: "muted" }, "A folder Google Drive, OneDrive, Dropbox or iCloud keeps in sync on all your PCs."), cloud.el];
      case 2: return [h("h1", {}, "What should this PC be called?"),
        h("p", { class: "muted" }, "Your other PCs show this name."), name];
      case 3: return hasData
        ? [h("h1", {}, "Is this your first PC with wtfsync?"),
          pcs.length
            ? h("p", { class: "muted" }, "This cloud folder already has settings from ", h("b", {}, pcs.join(", ")), ".")
            : h("p", { class: "muted" }, "This cloud folder already has wtfsync settings from another PC."),
          choice(true, "This is my first PC", "Start a new sync with this PC's settings."),
          choice(false, pcs.length ? "Join " + pcs.join(", ") : "Join my other PC",
            "Get their settings. If both PCs changed the same file, you'll choose which to keep.")]
        : [h("h1", {}, "Start syncing from this PC"),
          h("p", { class: "muted" }, "This cloud folder has no wtfsync data yet, so this PC starts it. Set up your other PCs afterwards and choose to join this one.")];
      default: return result
        ? [h("h1", {}, d.Rerun ? "Saved" : "This PC is set up"), h("ul", { class: "lines" }, result.map((l) => h("li", {}, l)))]
        : [h("h1", {}, "Setting up…"), h("p", { class: "muted" }, "The first sync can take a minute.")];
    }
  }

  function buttons() {
    if (step === 4) return result ? [h("span", { class: "grow" }), h("button", { class: "primary", onclick: done }, "Finish")] : [];
    return [
      step > 0 ? h("button", { onclick: () => { error = ""; step--; draw(); } }, "← Back") : "",
      h("span", { class: "grow" }),
      h("button", { onclick: cancel }, "Cancel"),
      h("button", { class: "primary", onclick: next }, step === 3 ? "Set up" : "Next"),
    ];
  }

  function draw() {
    root.replaceChildren(
      h("div", { class: "steps" }, STEPS.map((s, i) => [
        i ? h("span", { class: "ln" }) : "",
        h("span", { class: i < step ? "done" : i === step ? "cur" : "" }, h("span", { class: "n" }, i < step ? "✓" : String(i + 1)), s),
      ])),
      ...body(),
      error ? h("p", { class: "error" }, error) : "",
      h("div", { class: "row end" }, buttons()));
  }

  draw();
}
