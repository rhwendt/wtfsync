// Shared helpers for wtfsync's windows. Go methods live on ui.Bridge (internal/ui/bridge_windows.go).
import { Call, Window, Browser } from "/wails/runtime.js";

const bridge = "github.com/rhwendt/wtfsync/internal/ui.Bridge.";

export const call = (method, ...args) => Call.ByName(bridge + method, ...args);
export const close = () => Window.Close();
export const openURL = (url) => call("OpenURL", url);

// h builds an element: h("p", {class: "x"}, "text", child, ...). Strings become text nodes.
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false && v != null) el.setAttribute(k, v);
  }
  for (const c of children.flat()) el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  return el;
}

export const $ = (sel) => document.querySelector(sel);

// show replaces #main's content.
export function show(...children) {
  $("#main").replaceChildren(...children);
}

export function errorText(e) {
  return (e && (e.message || e.toString())) || "Something went wrong";
}

// busy disables a button while fn runs, then shows its result or error in #msg.
export async function busy(button, fn) {
  button.disabled = true;
  $("#msg").textContent = "";
  try {
    return await fn();
  } catch (e) {
    $("#msg").textContent = errorText(e);
    $("#msg").className = "error";
  } finally {
    button.disabled = false;
  }
}

// folderPicker is a <select> of options plus "Browse…"; returns {el, value()}.
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
  return { el: h("div", { class: "row" }, sel, browse), select: sel, value: () => sel.value };
}
