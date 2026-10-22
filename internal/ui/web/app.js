// Shared helpers for wtfsync's pages. Go methods live on ui.Bridge (internal/ui/bridge_windows.go).
// Opened with ?demo outside the app, pages use demo.js's sample data instead (for screenshots).
export const params = new URLSearchParams(location.search);
const demo = params.has("demo");
const rt = demo ? null : await import("/wails/runtime.js");
const fake = demo ? await import("./demo.js") : null;

const bridge = "github.com/rhwendt/wtfsync/internal/ui.Bridge.";

export const call = (method, ...args) => (demo ? fake.call(method, ...args) : rt.Call.ByName(bridge + method, ...args));
export const close = () => { if (!demo) rt.Window.Close(); };
export const on = (event, fn) => { if (!demo) rt.Events.On(event, (ev) => fn(ev.data)); };
export const openURL = (url) => call("OpenURL", url);

// applyTheme sets "light" or "dark", or follows Windows for "system".
export function applyTheme(appearance) {
  const t = params.get("theme") || appearance;
  if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
  else delete document.documentElement.dataset.theme;
}

// h builds an element: h("p", {class: "x"}, "text", child, [more]). Strings become text nodes;
// "" / null / false children are skipped.
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "checked" || k === "disabled" || k === "hidden") el[k] = !!v;
    else if (k === "value") el.value = v;
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false && v != null) el.setAttribute(k, v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === "" || c == null || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}

export const $ = (sel) => document.querySelector(sel);

export function errorText(e) {
  return (e && (e.message || e.toString())) || "Something went wrong";
}

export const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

// folderPicker is a <select> of options plus "Browse…".
export function folderPicker(options, current, title) {
  const sel = h("select", {});
  const add = (v) => { if (v && ![...sel.options].some((o) => o.value === v)) sel.append(h("option", { value: v }, v)); };
  options.forEach(add);
  add(current);
  sel.value = current || options[0] || "";
  const browse = h("button", { type: "button", onclick: async () => {
    const dir = await call("Browse", title);
    if (dir) { add(dir); sel.value = dir; sel.dispatchEvent(new Event("change")); }
  } }, "Browse…");
  return { el: h("div", { class: "row grow" }, sel, browse), select: sel, value: () => sel.value };
}
