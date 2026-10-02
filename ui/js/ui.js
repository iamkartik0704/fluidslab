// DAMBREAK — tiny DOM helpers for the neo-brutalist component classes.

import { icon } from "./icons.js";

/** el(tag, className, text) -> Element */
export function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

/**
 * chip(text, variant, iconName) -> HTMLSpanElement
 * variant: black | blue | yellow | orange | green | cream | "" (white)
 */
export function chip(text, variant = "", iconName = null) {
  const c = el("span", "chip" + (variant ? ` chip--${variant}` : ""));
  if (iconName) {
    const wrap = el("span");
    wrap.innerHTML = icon(iconName);
    const svg = wrap.firstChild;
    svg.classList.add("chip__icon");
    svg.style.width = "12px";
    svg.style.height = "12px";
    c.appendChild(svg);
  }
  c.appendChild(document.createTextNode(text));
  return c;
}

/** panelHead(eyebrowText, titleText) -> {head, titleEl} */
export function panelHead(eyebrowText, titleText) {
  const head = el("div", "panel__head");
  const left = el("div");
  left.appendChild(el("div", "eyebrow", eyebrowText));
  const titleEl = el("h2", "panel__title panel-title", titleText);
  left.appendChild(titleEl);
  head.appendChild(left);
  return { head, titleEl };
}

/** kpi(label, id, variant) -> HTMLDivElement with a .kpi__number span #id */
export function kpi(label, id, variant = "") {
  const k = el("div", "kpi" + (variant ? ` kpi--${variant}` : ""));
  k.appendChild(el("div", "kpi__label", label));
  const n = el("div", "kpi__number", "–");
  if (id) n.id = id;
  k.appendChild(n);
  return k;
}

/** Toasts: stacked top-right, auto-dismiss. */
export function toast(message, variant = "black", ms = 3500) {
  let host = document.getElementById("toastHost");
  if (!host) {
    host = el("div");
    host.id = "toastHost";
    host.style.cssText =
      "position:fixed;top:16px;right:16px;z-index:100;display:flex;flex-direction:column;gap:10px;max-width:min(90vw,420px)";
    document.body.appendChild(host);
  }
  const t = el("div", `chip chip--${variant}`);
  t.style.cssText = "box-shadow:4px 4px 0 var(--ink);padding:10px 14px;font-size:12px;white-space:normal";
  t.textContent = message;
  host.appendChild(t);
  setTimeout(() => t.remove(), ms);
}

/** WS/solver state -> chip variant mapping. */
export function stateChipVariant(state) {
  switch (state) {
    case "CONNECTED":    return "green";
    case "RECONNECTING": return "orange";
    case "PAUSED":       return "yellow";
    case "RUNNING":      return "blue";
    case "ERROR":        return "orange";
    default:             return "black";
  }
}
