// The conflict dialog: one window per question; closing it cancels that question (ui.openConflicts).
import { $, h, call, close, applyTheme } from "./app.js";

const show = (...children) => $("#main").replaceChildren(...children);
applyTheme((await call("Main"))?.Appearance || "system");
const req = await call("Conflicts");
// Answer before closing: the window's close event cancels this question if it's still open.
const reply = async (choices, cancel) => { await call("AnswerConflicts", req.ID, choices, cancel); close(); };
const title = req.Flavor ? `${req.Flavor}: settings changed on both PCs` : "Settings changed on both PCs";
const all = (c) => Object.fromEntries(req.Files.map((f) => [f, c]));
const footer = h("p", { class: "muted" }, "Cancel changes nothing. Files you replace are backed up first.");

function perFile() {
  const rows = req.Files.map((f) => {
    const pick = { value: "remote" };
    const seg = h("span", { class: "seg" }, [["remote", `${req.Other}'s`], ["local", "this PC's"]].map(([v, label]) =>
      h("label", {}, h("input", { type: "radio", name: f, checked: v === "remote", onchange: () => { pick.value = v; } }), label)));
    return { f, pick, seg, tr: h("tr", {}, h("td", {}, h("code", {}, f)), h("td", {}, seg)) };
  });
  const setAll = (v) => rows.forEach((r) => {
    r.pick.value = v;
    r.seg.querySelectorAll("input").forEach((i, n) => { i.checked = (n === 0) === (v === "remote"); });
  });
  show(
    h("h1", {}, title),
    h("p", {}, `These files changed on `, h("b", {}, req.Other), ` and on this PC since you last synced. Choose which version to keep.`),
    h("table", {}, rows.map((r) => r.tr)),
    h("div", { class: "row" }, h("span", { class: "muted" }, "Set all to"),
      h("button", { onclick: () => setAll("remote") }, `${req.Other}'s`),
      h("button", { onclick: () => setAll("local") }, "this PC's"),
      h("span", { class: "grow" }),
      h("button", { onclick: () => reply(null, true) }, "Cancel"),
      h("button", { class: "primary", onclick: () => reply(Object.fromEntries(rows.map((r) => [r.f, r.pick.value])), false) }, "Apply")),
    footer);
}

if (!req.Files || req.Files.length === 0) {
  show(h("p", { class: "muted" }, "Nothing to decide."), h("div", { class: "row end" }, h("button", { onclick: close }, "Close")));
} else if (req.FirstLink) {
  show(
    h("h1", {}, title),
    h("p", {}, `This PC and `, h("b", {}, req.Other), ` both have settings for ${req.Files.length} file(s).`),
    h("div", { class: "row end" },
      h("button", { onclick: () => reply(null, true) }, "Cancel"),
      h("button", { onclick: perFile }, "Choose per file…"),
      h("button", { onclick: () => reply(all("local"), false) }, "Keep this PC's"),
      h("button", { class: "primary", onclick: () => reply(all("remote"), false) }, `Use ${req.Other}'s for all`)),
    footer);
} else {
  perFile();
}
