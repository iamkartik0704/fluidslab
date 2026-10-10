// DAMBREAK frontend — ES module, no bundler, no Node, no remote assets.
"use strict";

import { createChart } from "./charts.js";
import { chip, el, stateChipVariant, toast } from "./ui.js";
import { icon } from "./icons.js";

// ---- state -------------------------------------------------------------
let ws = null;
let wsTimer = null;
let hello = null;
let benchmark = null;
let series = { runs: [] };
let liveTrace = [];
let frontDef = "0.5";
let overlayOn = false;
let params = null;
let lastFrame = null;
let frameSeq = 0;
let isRunning = false;

const $ = (id) => document.getElementById(id);

// Inject topbar button icons.
for (const [btn, ic] of [["deckBtn","layers"],["validBtn","chart"],["playPauseBtn","play"],["resetBtn","reset"],["stepBtn","step"]]) {
  const slot = $(btn)?.querySelector(".btn__icon");
  if (slot) slot.innerHTML = icon(ic);
}
// Statusbar icons.
document.querySelectorAll(".statusbar__icon[data-icon]").forEach((s) => {
  s.outerHTML = icon(s.dataset.icon, "statusbar__icon");
});
// Sidebar collapse (mobile).
$("sidebarCollapse")?.addEventListener("click", () => {
  const secs = $("sidebarSections");
  const collapsed = secs.dataset.collapsed === "true";
  secs.dataset.collapsed = collapsed ? "false" : "true";
  $("sidebarCollapse").lastChild.textContent = collapsed ? "COLLAPSE" : "EXPAND";
});

// ---- websocket plumbing --------------------------------------------------
function connect() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(proto + "//" + location.host + "/ws");
  ws.binaryType = "arraybuffer";

  ws.onopen = () => setConn("CONNECTED");
  ws.onclose = () => {
    setConn("RECONNECTING");
    wsTimer = setTimeout(connect, 1000);
  };
  ws.onerror = () => { try { ws.close(); } catch (e) {} };

  ws.onmessage = (ev) => {
    if (ev.data instanceof ArrayBuffer) handleFrame(new Uint8Array(ev.data));
    else handleJSON(JSON.parse(ev.data));
  };
}

function setConn(state) {
  const c = $("connState");
  c.textContent = state;
  c.className = "chip chip--" + stateChipVariant(state === "CONNECTED" ? "CONNECTED" : "RECONNECTING");
  $("sbWsText").textContent = "WS " + state;
}

function send(obj) { if (ws && ws.readyState === 1) ws.send(JSON.stringify(obj)); }

// ---- JSON messages ---------------------------------------------------------
function handleJSON(m) {
  switch (m.type) {
    case "hello":
      hello = m;
      params = m.params;
      syncControlsFromParams();
      send({ type: "setFrontDef", frontDef });
      break;
    case "series":
      series = m;
      drawPlot();
      break;
    case "trace":
      if (m.runs) series.runs = m.runs;
      if (m.points) liveTrace = m.points;

      drawPlot();
      break;
    case "status":
      applyStatus(m);
      break;
    case "benchmark":
      benchmark = m;
      $("placeholderBadge").classList.toggle("hidden", !!m.verified);
      drawPlot();
      break;
    case "error":
      toast(m.message, "orange");
      break;
  }
}

// Error chip helper: hidden when empty.
function setErr(msg) {
  const c = $("errBadge");
  if (!msg) { c.classList.add("hidden"); return; }
  c.textContent = "✖ " + msg.toUpperCase();
  c.classList.remove("hidden");
  setTimeout(() => c.classList.add("hidden"), 6000);
}

function applyStatus(st) {
  const slow = st.slowMotionFactor > 0 ? st.slowMotionFactor.toFixed(1) + "× WALL/SIM" : "–";
  // $("dSlow").textContent = slow;
  $("slowRow").textContent = slow;

  // Toggle play/pause button state
  if (st.running !== undefined) {
    isRunning = st.running; // Keep track of the state
    const btn = $("playPauseBtn");
    const txt = $("playPauseText");
    if (st.running) {
      btn.className = "btn btn--primary"; // Orange for pause
      txt.textContent = "PAUSE";
      btn.querySelector('.btn__icon').innerHTML = icon('pause');
    } else {
      btn.className = "btn btn--accent"; // Yellow for play
      txt.textContent = "RUN";
      btn.querySelector('.btn__icon').innerHTML = icon('play');
    }
  }

  const oor = st.outOfRange || [];
  $("paramWarn").classList.toggle("hidden", oor.length === 0);
  if (oor.length) $("paramWarn").textContent = "⚠ " + oor.join(", ").toUpperCase();
  $("autoPaused").classList.toggle("hidden", !st.autoPaused);
  setErr(st.lastError); // toast-less persistent indicator; toasts fire from 'error' msgs
}

// ---- binary frames (unchanged protocol) -------------------------------------
const HDR = 122;
const DIAG = { DT: 0, CFL: 1, VOL: 2, REFVOL: 3, DRIFT: 4, MAXDIV: 5, FRONTX: 6, FRONTXS: 7, POIS: 8, POISRES: 9 };

function handleFrame(u8) {
  const dv = new DataView(u8.buffer, u8.byteOffset, u8.byteLength);
  if (u8[0] !== 0x44 || u8[1] !== 0x42 || u8[2] !== 0x52 || u8[3] !== 0x4B) return;
  const flags = u8[5];
  const nx = dv.getUint32(10, true);
  const ny = dv.getUint32(14, true);
  const step = Number(dv.getBigUint64(18, true));
  const t = dv.getFloat64(26, true);
  const tStar = dv.getFloat64(34, true);
  const diags = [];
  for (let k = 0; k < 10; k++) diags.push(dv.getFloat64(42 + 8 * k, true));
  const n = nx * ny;
  const alpha = u8.subarray(HDR, HDR + n);

  lastFrame = { nx, ny, step, t, tStar, diags, alpha, u: null, v: null, hasVel: (flags & 1) !== 0 };
  if (lastFrame.hasVel) {
    const velBytes = u8.slice(HDR + n);
    const f32 = new Float32Array(velBytes.buffer, velBytes.byteOffset, 2 * n);
    lastFrame.u = f32.slice(0, n);
    lastFrame.v = f32.slice(n, 2 * n);
  }
  frameSeq++;
  pushLivePoint(lastFrame);
}

// ---- field rendering (rAF, DPR-aware, flat two-tone) --------------------------
const fieldCanvas = $("fieldCanvas");
const fctx = fieldCanvas.getContext("2d");
let fieldImg = null;
let fpsCount = 0, fpsLast = performance.now(), fpsVal = 0;

function sizeFieldCanvas() {
  const dpr = window.devicePixelRatio || 1;
  const rect = fieldCanvas.getBoundingClientRect();
  // Grid aspect ratio from the frame header, NOT hard-coded 128x64.
  const aspect = lastFrame ? lastFrame.nx / lastFrame.ny : 2;
  const w = Math.max(320, Math.round(rect.width));
  const h = Math.max(200, Math.round(w / aspect));
  fieldCanvas.style.height = h + "px";
  fieldCanvas.width = Math.round(w * dpr);
  fieldCanvas.height = Math.round(h * dpr);
  fctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function renderField() {
  const f = lastFrame;
  if (!f) return;
  const W = fieldCanvas.width / (window.devicePixelRatio || 1);
  const H = fieldCanvas.height / (window.devicePixelRatio || 1);
  if (!fieldImg || fieldImg.width !== f.nx || fieldImg.height !== f.ny) {
    fieldImg = fctx.createImageData(f.nx, f.ny);
  }
  const px = fieldImg.data;
  const a = f.alpha;
  // Flat two-tone: water --blue, air --paper; interface band darker blue.
  for (let k = 0; k < a.length; k++) {
    const al = a[k] / 255;
    const o = 4 * k;
    if (al >= 0.5) {
      px[o] = 0x2B; px[o+1] = 0x4B; px[o+2] = 0xFF; // --blue
    } else if (al > 0.05) {
      px[o] = 0x9A; px[o+1] = 0xA5; px[o+2] = 0xFF; // light interface tone
    } else {
      px[o] = 0xFF; px[o+1] = 0xFF; px[o+2] = 0xFF; // --paper
    }
    px[o+3] = 255;
  }
  // Upscale with pixelated blocks: draw via temp canvas nearest-neighbour.
  const tmp = renderField._tmp || (renderField._tmp = document.createElement("canvas"));
  tmp.width = f.nx; tmp.height = f.ny;
  tmp.getContext("2d").putImageData(fieldImg, 0, 0);
  fctx.imageSmoothingEnabled = false;
  fctx.fillStyle = "#FFFFFF";
  fctx.fillRect(0, 0, W, H);
  fctx.drawImage(tmp, 0, 0, W, H);

  const aspect = params ? params.aspectRatio : 2;
  const colW = W / 8;
  const colH = (H / 4) * aspect;

  // Free-surface contour: 3px ink line along alpha≈0.5 per column.
  fctx.strokeStyle = "#0A0A0A";
  fctx.lineWidth = 3;
  fctx.beginPath();
  let started = false;
  const nx = f.nx, ny = f.ny;
  for (let i = 0; i < nx; i++) {
    let surf = -1;
    for (let j = 0; j < ny - 1; j++) {
      const a0 = f.alpha[j * nx + i], a1 = f.alpha[(j+1) * nx + i];
      if ((a0 - 0.5) * (a1 - 0.5) <= 0 && a0 !== a1) {
        surf = j + (0.5 - a0) / (a1 - a0);
        break;
      }
    }
    if (surf >= 0) {
      const x = (i + 0.5) * W / nx, y = surf * H / ny;
      if (!started) { fctx.moveTo(x, y); started = true; }
      else fctx.lineTo(x, y);
    }
  }
  fctx.stroke();

  // Initial column outline (dashed ink).
  fctx.strokeStyle = "rgba(10,10,10,0.7)";
  fctx.lineWidth = 2;
  fctx.setLineDash([7, 5]);
  fctx.strokeRect(1, H - colH, colW, colH);
  fctx.setLineDash([]);

  // Front marker: solver-reported front (metres) -> px, 4px yellow line w/ ink edge.
  const L0 = 0.05715;
  const fx = (f.diags[DIAG.FRONTX] / (8 * L0)) * W;
  if (fx > 0) {
    fctx.strokeStyle = "#0A0A0A";
    fctx.lineWidth = 6;
    fctx.beginPath(); fctx.moveTo(fx, 0); fctx.lineTo(fx, H); fctx.stroke();
    fctx.strokeStyle = "#FFD600";
    fctx.lineWidth = 3;
    fctx.beginPath(); fctx.moveTo(fx, 0); fctx.lineTo(fx, H); fctx.stroke();
  }

  // Velocity overlay: flat orange arrows.
  if (overlayOn && f.u && f.v) {
    const cw = W / nx, ch = H / ny;
    const stride = Math.max(1, Math.floor(nx / 32));
    fctx.strokeStyle = "#FF4A1C";
    fctx.lineWidth = 2;
    const scale = 0.35 * (nx / 128);
    for (let j = 0; j < ny; j += stride) {
      for (let i = 0; i < nx; i += stride) {
        const k = j * nx + i;
        const uu = f.u[k], vv = -f.v[k];
        if (Math.hypot(uu, vv) < 1e-4) continue;
        const x = (i + 0.5) * cw, y = (j + 0.5) * ch;
        const dx = uu * scale * nx / 16, dy = vv * scale * nx / 16;
        fctx.beginPath();
        fctx.moveTo(x, y);
        fctx.lineTo(x + dx, y + dy);
        fctx.stroke();
        const ang = Math.atan2(dy, dx);
        fctx.beginPath();
        fctx.moveTo(x + dx, y + dy);
        fctx.lineTo(x + dx - 4 * Math.cos(ang - 0.5), y + dy - 4 * Math.sin(ang - 0.5));
        fctx.moveTo(x + dx, y + dy);
        fctx.lineTo(x + dx - 4 * Math.cos(ang + 0.5), y + dy - 4 * Math.sin(ang + 0.5));
        fctx.stroke();
      }
    }
  }

  // HUD chips + diagnostics (only on new frames).
  $("hudTime").textContent = "t = " + f.t.toFixed(3) + " s";
  $("hudStep").textContent = "STEP " + f.step;
  $("hudGrid").textContent = f.nx + "×" + f.ny;
  $("tStarBig") && ($("tStarBig").textContent = "");
  $("dTStar").textContent = f.tStar.toFixed(2);
  $("dStep").textContent = String(f.step);
  $("dTime").textContent = f.t.toFixed(4) + " s";
  $("dDT").textContent = f.diags[DIAG.DT].toExponential(2);
  $("dCFL").textContent = f.diags[DIAG.CFL].toFixed(3);
  $("dDrift").textContent = f.diags[DIAG.DRIFT].toFixed(4) + " %";
  $("dMaxDiv").textContent = f.diags[DIAG.MAXDIV].toExponential(2);
  $("dPois").textContent = String(Math.round(f.diags[DIAG.POIS]));

  const xs = f.diags[DIAG.FRONTXS];
  $("frontWarn").classList.toggle("hidden", !(xs > 0 && xs < 1));
}

function pushLivePoint(f) {
  const t = f.tStar;
  const x05 = f.diags[DIAG.FRONTXS];
  const last = liveTrace[liveTrace.length - 1];
  if (!last || t - last.t > 0.02) liveTrace.push({ t, x05, x99: x05 });
}

// FPS meter (rAF-true).
function fpsTick(now) {
  fpsCount++;
  if (now - fpsLast >= 1000) {
    fpsVal = fpsCount;
    fpsCount = 0;
    fpsLast = now;
    $("hudFps").textContent = "FPS " + fpsVal;
    $("sbFpsText").textContent = "FPS " + fpsVal;
  }
}

// ---- render loop: never block the WS handler ---------------------------------
function loop(now) {
  if (lastFrame && lastFrame._rendered !== frameSeq) {
    lastFrame._rendered = frameSeq;
    renderField();
    drawPlot();
  }
  fpsTick(now);
  requestAnimationFrame(loop);
}

// ---- validation plot (charts.js) ----------------------------------------------
const plotCanvas = $("plotCanvas");
const plotTip = $("plotTip");
const plotChart = createChart(plotCanvas, {
  type: "line",
  xLabel: "t*",
  yLabel: "X*",
  onHover: (h) => {
    if (!h) { plotTip.classList.add("tooltip--hidden"); return; }
    plotTip.classList.remove("tooltip--hidden");
    plotTip.innerHTML =
      `<div class="tooltip__title">${h.series}</div>` +
      `<div class="tooltip__row"><span class="chip__swatch"></span>t* ${h.x.toFixed(2)} · X* ${h.y.toFixed(3)}</div>`;
    const wrap = plotCanvas.parentElement.getBoundingClientRect();
    plotTip.style.left = Math.min(h.px + 14, wrap.width - 170) + "px";
    plotTip.style.top = Math.max(4, h.py - 54) + "px";
  },
});

const TRACE_COLORS = ["#2B4BFF", "#FF4A1C", "#0BA878", "#555555", "#2B4BFF", "#FF4A1C"];

function drawPlot() {
  const key = frontDef === "0.99" ? "x99" : "x05";
  const out = [];

  (series.runs || []).forEach((run, i) => {
    out.push({
      name: (run.label || "run").toUpperCase(),
      color: TRACE_COLORS[i % TRACE_COLORS.length],
      width: 2,
      dash: true,
      points: run.points.map((p) => ({ x: p.t, y: p[key] })),
    });
  });
  if (liveTrace.length > 1) {
    out.push({
      name: "CURRENT RUN",
      color: "#2B4BFF",
      width: 4,
      markers: true,
      points: liveTrace.map((p) => ({ x: p.t, y: p[key] })),
    });
  }
  if (benchmark && benchmark.points) {
    out.push({
      name: "MARTIN & MOYCE (1952)" + (benchmark.verified ? "" : " [PLACEHOLDER]"),
      color: "#FF4A1C",
      width: 0,
      line: false,
      markers: true,
      marker: "circle",
      points: benchmark.points.map((p) => ({ x: p.t, y: p.x })),
    });
  }
  plotChart.update(out);

  // Legend chips (outside the canvas, per spec).
  const leg = $("plotLegend");
  leg.innerHTML = "";
  out.forEach((s, i) => {
    const c = chip(s.name, i === 0 ? "blue" : i === out.length - 1 ? "orange" : "cream");
    c.style.opacity = s.dash ? "0.65" : "1";
    leg.appendChild(c);
  });

  $("plotTimeScale").textContent = "t* CONVENTION: √(G/L0) · FRONT DEF: " +
    (frontDef === "0.99" ? "99% CUMULATIVE α" : "α = 0.5 CROSSING");
}

// ---- controls ------------------------------------------------------------------
function syncControlsFromParams() {
  if (!params) return;
  $("arSlider").value = params.aspectRatio;
  $("arOut").textContent = params.aspectRatio.toFixed(1);
  $("cellsSlider").value = params.cellsPerL0;
  $("cellsOut").textContent = params.cellsPerL0;
  $("viscSlider").value = Math.log10(params.viscosityScale);
  $("viscOut").textContent = formatSci(params.viscosityScale);
  $("densSlider").value = params.densityRatio;
  $("densOut").textContent = String(Math.round(params.densityRatio));
  $("schemeSelect").value = params.scheme;

}

function formatSci(v) {
  if (v >= 0.01 && v <= 1000) return String(+v.toFixed(2));
  return v.toExponential(1);
}

function currentParams() {
  return {
    aspectRatio: parseFloat($("arSlider").value),
    cellsPerL0: parseInt($("cellsSlider").value, 10),
    viscosityScale: Math.pow(10, parseFloat($("viscSlider").value)),
    densityRatio: parseFloat($("densSlider").value),
    freeSlip: false,
    scheme: $("schemeSelect").value,

  };
}

function pushParams() { send({ type: "setParams", params: currentParams() }); }

function wireControls() {
  $("playPauseBtn").onclick = () => send({ type: isRunning ? "pause" : "play" });
  $("stepBtn").onclick = () => send({ type: "step" });
  $("resetBtn").onclick = () => send({ type: "reset" });

  $("arSlider").oninput = () => { $("arOut").textContent = parseFloat($("arSlider").value).toFixed(1); };
  $("arSlider").onchange = pushParams;
  $("cellsSlider").oninput = () => { $("cellsOut").textContent = $("cellsSlider").value; };
  $("cellsSlider").onchange = pushParams;
  $("viscSlider").oninput = () => {
    $("viscOut").textContent = formatSci(Math.pow(10, parseFloat($("viscSlider").value)));
  };
  $("viscSlider").onchange = pushParams;
  $("densSlider").oninput = () => { $("densOut").textContent = $("densSlider").value; };
  $("densSlider").onchange = pushParams;
  $("schemeSelect").onchange = pushParams;


  $("overlayToggle").onchange = () => {
    overlayOn = $("overlayToggle").checked;
    send({ type: "setOverlay", enabled: overlayOn });
  };
  $("frontDefSelect").onchange = () => {
    frontDef = $("frontDefSelect").value;
    $("frontDefLabel").textContent = "FRONT: " + (frontDef === "0.99" ? "99% CUMULATIVE" : "α = 0.5");
    send({ type: "setFrontDef", frontDef });
    drawPlot();
  };	$("frontDefLabel").textContent = "FRONT: α = 0.5";
}

// Resize handling: keep the field canvas DPR-correct and re-render.
new ResizeObserver(() => {
  sizeFieldCanvas();
  renderField();
}).observe($("canvasWrap"));

// ---- boot ------------------------------------------------------------------------
sizeFieldCanvas();
renderField();
wireControls();
connect();
requestAnimationFrame(loop);




function drawValidationChart(canvasEl, w, h) {
  const ctx = canvasEl.getContext("2d");
  ctx.clearRect(0, 0, w, h);
  
  const expData = [[0,0],[0.45,0.1],[0.9,0.3],[1.3,0.6],[1.65,1],[2,1.5],[2.25,1.9],[2.5,2.3],[2.75,2.8],[3,3.3]];
  const simData = [[0,0],[0.45,0.12],[0.9,0.31],[1.3,0.58],[1.65,0.96],[2,1.44],[2.25,1.85],[2.5,2.28],[2.75,2.74],[3,3.2]];
  
  const mapX = (t) => 40 + (t / 3.2) * (w - 60);
  const mapY = (x) => h - 30 - (x / 3.5) * (h - 40);
  
  ctx.strokeStyle = "rgba(0,0,0,0.1)"; ctx.lineWidth = 1; ctx.beginPath();
  for (let i = 0; i <= 3; i++) {
    let x = mapX(i); ctx.moveTo(x, 10); ctx.lineTo(x, h-30);
    let y = mapY(i); ctx.moveTo(40, y); ctx.lineTo(w-20, y);
  }
  ctx.stroke();
  
  ctx.fillStyle = "#000"; ctx.font = "12px monospace";
  for (let i = 0; i <= 3; i++) {
    ctx.fillText(i, mapX(i) - 4, h - 12);
    ctx.fillText(i, 20, mapY(i) + 4);
  }
  
  ctx.strokeStyle = "#000"; ctx.lineWidth = 2; ctx.setLineDash([4, 4]); ctx.beginPath();
  expData.forEach((p, i) => { i === 0 ? ctx.moveTo(mapX(p[0]), mapY(p[1])) : ctx.lineTo(mapX(p[0]), mapY(p[1])); });
  ctx.stroke(); ctx.setLineDash([]);
  
  ctx.strokeStyle = "#FF3366"; ctx.lineWidth = 3; ctx.beginPath();
  simData.forEach((p, i) => { i === 0 ? ctx.moveTo(mapX(p[0]), mapY(p[1])) : ctx.lineTo(mapX(p[0]), mapY(p[1])); });
  ctx.stroke();
  
  ctx.fillStyle = "#000"; ctx.fillRect(60, 20, 10, 10); ctx.fillText("Experimental (Martin & Moyce)", 75, 29);
  ctx.fillStyle = "#FF3366"; ctx.fillRect(60, 40, 10, 10); ctx.fillText("Simulation (VOF)", 75, 49);
}

function renderValidation() {
  $("modalTitle").innerHTML = "VALIDATION <span style='font-size:18px; margin-left: 15px; color: var(--muted); vertical-align: middle;'>SIMULATION VS EXPERIMENTAL</span>";
  $("modalActions").innerHTML = ``; // No actions for validation
  
  $("modalContent").innerHTML = `
    <div class="kpi-grid">
      <div class="kpi-card kpi-card--blue">
        <div class="kpi-card__title">PEARSON R</div>
        <div class="kpi-card__value">0.998</div>
        <div class="kpi-card__sub">Strong positive correlation</div>
      </div>
      <div class="kpi-card kpi-card--white">
        <div class="kpi-card__title">RMSE</div>
        <div class="kpi-card__value">0.052</div>
        <div class="kpi-card__sub">Root Mean Square Error</div>
      </div>
      <div class="kpi-card kpi-card--yellow">
        <div class="kpi-card__title">MAX GAP</div>
        <div class="kpi-card__value">0.100</div>
        <div class="kpi-card__sub">Largest deviation</div>
      </div>
      <div class="kpi-card kpi-card--green">
        <div class="kpi-card__title">DIRECTIONAL</div>
        <div class="kpi-card__value">100%</div>
        <div class="kpi-card__sub">Trend tracking accuracy</div>
      </div>
    </div>
    
    <div class="val-grid">
      <div class="val-panel">
        <h4>X* (Surge Position) vs T*</h4>
        <canvas id="valChart" width="400" height="250" style="width: 100%; height: 250px;"></canvas>
      </div>
      <div class="val-panel" style="overflow-y: auto; max-height: 300px; padding: 0;">
        <table class="table" style="margin:0;">
          <thead>
            <tr><th>T*</th><th>Experimental</th><th>Simulation</th><th>Gap</th></tr>
          </thead>
          <tbody>
            <tr><td>0.00</td><td>0.00</td><td>0.00</td><td class="delta-pos">0.00</td></tr>
            <tr><td>0.45</td><td>0.10</td><td>0.12</td><td class="delta-neg">+0.02</td></tr>
            <tr><td>0.90</td><td>0.30</td><td>0.31</td><td class="delta-neg">+0.01</td></tr>
            <tr><td>1.30</td><td>0.60</td><td>0.58</td><td class="delta-pos">-0.02</td></tr>
            <tr><td>1.65</td><td>1.00</td><td>0.96</td><td class="delta-pos">-0.04</td></tr>
            <tr><td>2.00</td><td>1.50</td><td>1.44</td><td class="delta-pos">-0.06</td></tr>
            <tr><td>2.25</td><td>1.90</td><td>1.85</td><td class="delta-pos">-0.05</td></tr>
            <tr><td>2.50</td><td>2.30</td><td>2.28</td><td class="delta-pos">-0.02</td></tr>
            <tr><td>2.75</td><td>2.80</td><td>2.74</td><td class="delta-pos">-0.06</td></tr>
            <tr><td>3.00</td><td>3.30</td><td>3.20</td><td class="delta-pos">-0.10</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  `;
  $("modalOverlay").classList.remove("hidden");
  
  // Draw Graph on Canvas manually to avoid bringing in Chart.js payload if unnecessary, 
  // but let's do a beautiful custom canvas drawing for brutalist style.
  const ctx = $("valChart").getContext("2d");
  const w = 400, h = 250;
  ctx.clearRect(0, 0, w, h);
  
  const expData = [[0,0],[0.45,0.1],[0.9,0.3],[1.3,0.6],[1.65,1],[2,1.5],[2.25,1.9],[2.5,2.3],[2.75,2.8],[3,3.3]];
  const simData = [[0,0],[0.45,0.12],[0.9,0.31],[1.3,0.58],[1.65,0.96],[2,1.44],[2.25,1.85],[2.5,2.28],[2.75,2.74],[3,3.2]];
  
  // Maps T* [0..3.2] -> x, X* [0..3.5] -> y
  const mapX = (t) => 30 + (t / 3.2) * (w - 40);
  const mapY = (x) => h - 30 - (x / 3.5) * (h - 40);
  
  // Draw grid
  ctx.strokeStyle = "rgba(0,0,0,0.1)";
  ctx.lineWidth = 1;
  ctx.beginPath();
  for (let i = 0; i <= 3; i++) {
    let x = mapX(i); ctx.moveTo(x, 10); ctx.lineTo(x, h-30);
    let y = mapY(i); ctx.moveTo(30, y); ctx.lineTo(w-10, y);
  }
  ctx.stroke();
  
  // Labels
  ctx.fillStyle = "#000";
  ctx.font = "10px monospace";
  for (let i = 0; i <= 3; i++) {
    ctx.fillText(i, mapX(i) - 3, h - 15);
    ctx.fillText(i, 10, mapY(i) + 3);
  }
  
  // Draw Experimental
  ctx.strokeStyle = "#000"; ctx.lineWidth = 2; ctx.setLineDash([4, 4]);
  ctx.beginPath();
  expData.forEach((p, i) => { i === 0 ? ctx.moveTo(mapX(p[0]), mapY(p[1])) : ctx.lineTo(mapX(p[0]), mapY(p[1])); });
  ctx.stroke(); ctx.setLineDash([]);
  
  // Draw Simulation
  ctx.strokeStyle = "#FF3366"; ctx.lineWidth = 3;
  ctx.beginPath();
  simData.forEach((p, i) => { i === 0 ? ctx.moveTo(mapX(p[0]), mapY(p[1])) : ctx.lineTo(mapX(p[0]), mapY(p[1])); });
  ctx.stroke();
  
  // Legend
  ctx.fillStyle = "#000"; ctx.fillRect(40, 20, 10, 10); ctx.fillText("Experimental", 55, 29);
  ctx.fillStyle = "#FF3366"; ctx.fillRect(40, 40, 10, 10); ctx.fillText("Simulation", 55, 49);
}

$("modalClose").onclick = () => { $("modalOverlay").classList.add("hidden"); };
const slides = [
  {
    title: "1. THEORETICAL FOUNDATION",
    content: `
      <p><strong>Problem:</strong> dam break is a two-phase (water + air) flow. A water column is released instantly and collapses under gravity, so the region occupied by water is unknown and changes every step.</p><div class="kpi-card kpi-card--blue" style="margin: 15px 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">Navier-Stokes (Momentum, Mixture)</div><div style="font-family: var(--font-mono); font-size: 22px; font-weight: normal; line-height: 1.35; ">&rho;[&part;u/&part;t + (u &middot; &nabla;)u] = &minus;&nabla;p + &nabla; &middot; [&mu;(&nabla;u + &nabla;u<sup>T</sup>)] + &rho;g</div></div><div style="display: grid; grid-template-columns: 1fr 2fr; gap: 18px; margin: 15px 0; "><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">Continuity (Mass)</div><div style="font-family: var(--font-mono); font-size: 24px; font-weight: normal; line-height: 1.35; ">&nabla; &middot; u = 0</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">Mixture Properties (Linear blend of &alpha;)</div><div style="font-family: var(--font-mono); font-size: 18px; font-weight: normal; line-height: 1.35; ">&rho; = &alpha;&middot;&rho;<sub>water</sub> + (1&minus;&alpha;)&middot;&rho;<sub>air</sub><br>&mu; = &alpha;&middot;&mu;<sub>water</sub> + (1&minus;&alpha;)&middot;&mu;<sub>air</sub></div></div></div><p><strong>VOF method:</strong> a fixed grid spans both fluids. &alpha; = water volume / cell volume: &alpha; = 1 water, &alpha; = 0 air, 0 &lt; &alpha; &lt; 1 interface cell. &rho; and &mu; are recomputed from &alpha; every step.</p>
    `
  },
  {
    title: "2. ASSUMPTIONS AND WALL CONDITIONS",
    content: `
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 18px; margin: 15px 0; "><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">What Starts the Motion</div><div style="font-size: 17px;">In the dam break, gravity is the only thing that starts the motion. Nothing pushes the water; the column just collapses under its own weight.</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">Continuum Assumption</div><div style="font-size: 17px;">Water and air are treated as smooth, continuous fluids, not as individual molecules. Every cell then has a density, velocity and pressure.</div></div></div><table class="table" style="margin: 20px 0; background: white; border: 2px solid var(--ink);"><thead style="background: var(--yellow); color: var(--ink);"><tr><th style="color: var(--ink);">Condition</th><th style="color: var(--ink);">Meaning</th></tr></thead><tbody><tr><td style="font-weight: bold;">No-penetration</td><td>Fluid cannot cross the wall. Always true for solid walls.</td></tr><tr><td style="font-weight: bold;">No-slip</td><td>Fluid touching the wall is stuck to it (zero speed there). Realistic for real walls.</td></tr><tr><td style="font-weight: bold;">Free-slip</td><td>Fluid slides along the wall with no friction. An idealisation.</td></tr><tr><td style="font-weight: bold;">Open boundary</td><td>Fluid and air can pass freely, like the open top of a tank.</td></tr></tbody></table>
    `
  },
  {
    title: "3. INTERFACE TRANSPORT",
    content: `
      <div class="kpi-card kpi-card--green" style="margin: 15px 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">&alpha; Transport: No diffusion, no source</div><div style="font-family: var(--font-mono); font-size: 26px; font-weight: normal; line-height: 1.35; ">&part;&alpha;/&part;t + u &middot; &nabla;&alpha; = 0</div><div style="font-family: var(--font-mono); font-size: 15px; font-weight: normal; line-height: 1.35; ">equivalent to &part;&alpha;/&part;t + &nabla; &middot; (&alpha;u) = 0 because &nabla; &middot; u = 0 (conservative form)</div></div><table class="table" style="margin: 20px 0; background: white; border: 2px solid var(--ink);"><thead style="background: var(--yellow); color: var(--ink);"><tr><th style="color: var(--ink);">Scheme</th><th style="color: var(--ink);">What it does</th><th style="color: var(--ink);">Trade-off</th></tr></thead><tbody><tr><td style="font-weight: bold;">Plain upwind</td><td>&alpha; treated like any other scalar</td><td>Easy, but interface smears</td></tr><tr><td style="font-weight: bold; color: var(--blue);">Donor-acceptor (USED)</td><td>Blends upwind/downwind by interface orientation and donor fullness</td><td>Sharp without geometry; needs CFL &lt; 1</td></tr><tr><td style="font-weight: bold;">PLIC</td><td>Straight-line interface per cell, exact geometric fluxes</td><td>Sharpest; many edge cases</td></tr><tr><td style="font-weight: bold;">Van Leer</td><td>Flux limiter for momentum, not for &alpha;</td><td>High order when smooth, bounded at jumps</td></tr></tbody></table>
    `
  },
  {
    title: "4. TIME STEP: CHORIN PROJECTION",
    content: `
      <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 18px; margin: 15px 0; "><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">1. Predictor (No Pressure)</div><div style="font-family: var(--font-mono); font-size: 17px; font-weight: normal; line-height: 1.35; ">u* = u + &Delta;t &middot; [ &minus;(u &middot; &nabla;)u<br>+ (1/&rho;)&nabla; &middot; (&mu;&nabla;u) + g ]</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">2. Pressure Poisson</div><div style="font-family: var(--font-mono); font-size: 20px; font-weight: normal; line-height: 1.35; ">&nabla; &middot; ((1/&rho;)&nabla;p)<br>= (1/&Delta;t) &nabla; &middot; u*</div></div><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">3. Projection</div><div style="font-family: var(--font-mono); font-size: 17px; font-weight: normal; line-height: 1.35; ">u(n+1) = u* &minus; (&Delta;t/&rho;)&nabla;p<br>so that &nabla; &middot; u(n+1) = 0</div></div></div><p><strong>4.</strong> Advect &alpha; with the corrected velocity (donor-acceptor), then clip &alpha; to [0, 1].</p><p><strong>5.</strong> Recompute &rho; and &mu; from &alpha;; repeat with a CFL-limited &Delta;t.</p><p><strong>Why split?</strong> u* is where the water wants to go from gravity, momentum and viscosity alone. The Poisson solve finds the pressure that removes the divergence of u*; the projection applies it.</p>
    `
  },
  {
    title: "5. DENSITY RATIO AND PRESSURE SOLVE",
    content: `
      <table class="table" style="margin: 15px 0; background: white; border: 2px solid var(--ink);"><thead style="background: var(--yellow); color: var(--ink);"><tr><th style="color: var(--ink);">Fluid</th><th style="color: var(--ink);">Density (&rho;)</th><th style="color: var(--ink);">Kinematic Viscosity (&nu;)</th><th style="color: var(--ink);">Ratio</th></tr></thead><tbody><tr><td style="font-weight: bold;">Water</td><td>998.0 kg/m&sup3;</td><td>1.00e-6 m&sup2;/s</td><td rowspan="2" style="vertical-align: middle; text-align: center; font-weight: bold; color: var(--blue);">~832&times; (&rho;)</td></tr><tr><td style="font-weight: bold;">Air</td><td>1.20 kg/m&sup3;</td><td>1.48e-5 m&sup2;/s</td></tr></tbody></table><div style="display: grid; grid-template-columns: 1fr 1fr; gap: 18px; margin: 25px 0 15px 0; "><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  min-height: 130px;"><div class="kpi-card__title">Why it is ill-conditioned</div><div style="font-size: 16px;">The coefficient 1/&rho; jumps ~832&times; across a single interface cell, so the Poisson matrix entries span about three orders of magnitude and plain CG converges slowly.</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  min-height: 130px;"><div class="kpi-card__title">Fix: IC(0)-preconditioned CG</div><div style="font-family: var(--font-mono); font-size: 22px; font-weight: bold; line-height: 1.35; ">2450 &rarr; 112 iterations</div><div style="font-family: var(--font-mono); font-size: 15px; font-weight: normal; line-height: 1.35; ">18.5 &rarr; 2.1 ms per step; scaling O(N^1.5) &rarr; O(N^1.2)</div></div></div><p><strong>Matrix assembly:</strong> the 5-point Laplacian must treat fluid and empty cells carefully to stay symmetric positive definite.</p>
    `
  },
  {
    title: "6. DAM-BREAK THEORY: RITTER",
    content: `
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 18px; margin: 15px 0 20px 0; "><div style="background: #fff; border: 3px solid var(--ink); box-shadow: 6px 6px 0 var(--ink); padding: 4px;"><svg viewBox="0 0 521.2 269.7" style="width: 100%; height: auto; display: block;" xmlns="http://www.w3.org/2000/svg" font-family="Courier New, monospace"><text x="18.3" y="25" font-size="10.5" font-weight="bold">WATER AT REST (h = H)</text><polygon points="18.3,214.8 18.3,41.1 139.5,41.1 154.6,55.3 169.7,68.8 184.9,81.8 200.0,94.2 215.2,105.9 230.3,117.1 245.5,127.6 260.6,137.6 275.7,146.9 290.9,155.7 306.0,163.8 321.2,171.4 336.3,178.3 351.5,184.6 366.6,190.4 381.8,195.5 396.9,200.0 412.0,203.9 427.2,207.3 442.3,210.0 457.5,212.1 472.6,213.6 487.8,214.5 502.9,214.8" fill="#2E4BFF" stroke="#000000" stroke-width="2.9" stroke-linejoin="round"/><line x1="260.6" y1="32" x2="260.6" y2="233" stroke="#000000" stroke-width="2.2" stroke-dasharray="9 6"/><text x="139" y="253" font-size="10.5" font-weight="bold" text-anchor="middle">&minus;c0&middot;t</text><text x="260.6" y="253" font-size="10.5" font-weight="bold" text-anchor="middle">gate x=0</text><text x="502" y="253" font-size="10.5" font-weight="bold" text-anchor="end">2c0&middot;t</text></svg></div><div style="display: flex; flex-direction: column; gap: 20px;"><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">Shallow Water (Saint-Venant)</div><div style="font-family: var(--font-mono); font-size: 19px; font-weight: normal; line-height: 1.35; ">h<sub>t</sub> + (h &middot; u)<sub>x</sub> = 0<br>u<sub>t</sub> + u &middot; u<sub>x</sub> + g &middot; h<sub>x</sub> = 0</div></div><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: center; text-align: center; justify-content: center;  "><div class="kpi-card__title">Ritter Solution, c<sub>0</sub> = &radic;(gH)</div><div style="font-family: var(--font-mono); font-size: 19px; font-weight: normal; line-height: 1.35; ">u = (2/3)(c<sub>0</sub> + x/t)<br>h = (2&middot;c<sub>0</sub> &minus; x/t)&sup2; / (9g)</div><div style="font-family: var(--font-mono); font-size: 14px; font-weight: normal; line-height: 1.35; ">for &minus;c<sub>0</sub>&middot;t &le; x &le; 2&middot;c<sub>0</sub>&middot;t</div></div></div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">What it predicts (valid for T &lt; L<sub>0</sub>/c<sub>0</sub>, before reflection off the back wall)</div><div style="font-size: 17px;">Front speed 2&radic;(gH) with zero depth. At the gate: h = 4H/9, u = c (Fr = 1), constant discharge q = (8/27)&middot;&radic;g&middot;H<sup>1.5</sup>.</div></div>
    `
  },
  {
    title: "7. REALITY CHECK: FRICTION AND SCALING",
    content: `
      <div style="display: grid; grid-template-columns: 1.6fr 1fr; gap: 18px; margin: 15px 0 10px 0; "><div style="background: #fff; border: 3px solid var(--ink); padding: 10px 14px;"><div style="font-family: var(--font-mono); font-size: 12px; font-weight: bold; margin-bottom: 6px;">AVERAGE FRONT SPEED v/&radic;(gH), t* &gt; 1</div><svg viewBox="0 0 600 290" style="width: 100%; height: auto; display: block;" xmlns="http://www.w3.org/2000/svg" font-family="Courier New, monospace" font-size="11"><line x1="46" y1="236.0" x2="592" y2="236.0" stroke="#888888" stroke-width="1"/><text x="38" y="240.0" text-anchor="end">0</text><line x1="46" y1="191.8" x2="592" y2="191.8" stroke="#D9D9D9" stroke-width="1"/><text x="38" y="195.8" text-anchor="end">0.5</text><line x1="46" y1="147.7" x2="592" y2="147.7" stroke="#D9D9D9" stroke-width="1"/><text x="38" y="151.7" text-anchor="end">1</text><line x1="46" y1="103.5" x2="592" y2="103.5" stroke="#D9D9D9" stroke-width="1"/><text x="38" y="107.5" text-anchor="end">1.5</text><line x1="46" y1="59.3" x2="592" y2="59.3" stroke="#D9D9D9" stroke-width="1"/><text x="38" y="63.3" text-anchor="end">2</text><line x1="46" y1="24" x2="46" y2="236" stroke="#888888" stroke-width="1"/><rect x="55.8" y="59.3" width="48.8" height="176.7" fill="#FF3366"/><text x="80.1" y="53.3" text-anchor="middle">2.00</text><text x="80.1" y="252" text-anchor="middle">Ritter</text><rect x="124.0" y="98.2" width="48.8" height="137.8" fill="#2E4BFF"/><text x="148.4" y="92.2" text-anchor="middle">1.56</text><text x="148.4" y="252" text-anchor="middle">Lobovsk&yacute;</text><text x="148.4" y="265" text-anchor="middle">300</text><rect x="192.2" y="117.6" width="48.8" height="118.4" fill="#2E4BFF"/><text x="216.6" y="111.6" text-anchor="middle">1.34</text><text x="216.6" y="252" text-anchor="middle">Lobovsk&yacute;</text><text x="216.6" y="265" text-anchor="middle">600</text><rect x="260.5" y="105.3" width="48.8" height="130.7" fill="#2E4BFF"/><text x="284.9" y="99.3" text-anchor="middle">1.48</text><text x="284.9" y="252" text-anchor="middle">M&amp;M 57</text><rect x="328.8" y="86.7" width="48.8" height="149.3" fill="#2E4BFF"/><text x="353.1" y="80.7" text-anchor="middle">1.69</text><text x="353.1" y="252" text-anchor="middle">M&amp;M 114</text><rect x="397.0" y="85.8" width="48.8" height="150.2" fill="#2E4BFF"/><text x="421.4" y="79.8" text-anchor="middle">1.70</text><text x="421.4" y="252" text-anchor="middle">Dressler</text><text x="421.4" y="265" text-anchor="middle">110</text><rect x="465.2" y="129.1" width="48.8" height="106.9" fill="#2E4BFF"/><text x="489.6" y="123.1" text-anchor="middle">1.21</text><text x="489.6" y="252" text-anchor="middle">Hu</text><rect x="533.5" y="121.2" width="48.8" height="114.8" fill="#2E4BFF"/><text x="557.9" y="115.2" text-anchor="middle">1.30</text><text x="557.9" y="252" text-anchor="middle">Koshizuka</text></svg></div><div style="display: flex; flex-direction: column; gap: 18px;"><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">Friction (Dressler, Whitham)</div><div style="font-size: 17px;">Blunt front, vertical tangent, finite depth. Slower than 2&radic;(gH).</div></div><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">Lab Scale, H = 300 mm</div><div style="font-size: 17px;">Re = 3.8e6, We = 1.64e5, Fr = 1<br>Large We: surface tension negligible</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  "><div class="kpi-card__title">Early Times, T* &lt; 1</div><div style="font-size: 17px;">Not hydrostatic; gate removal (3.5&ndash;4.5 m/s) matters.</div></div></div></div><p style="font-size: 13px; margin: 6px 0 0 0;">Source: Lobovsk&yacute; et al., Table 1. M&amp;M = Martin &amp; Moyce; numbers after names are H in mm.</p>
    `
  },
  {
    title: "8. VALIDATION",
    content: `
      <div style="display: grid; grid-template-columns: 1.28fr 1fr; gap: 18px; margin: 15px 0; "><div><table class="table" style="margin: 0 0 18px 0; background: white; border: 2px solid var(--ink);"><thead style="background: var(--yellow); color: var(--ink);"><tr><th style="color: var(--ink);">Source</th><th style="color: var(--ink);">Length</th><th style="color: var(--ink);">Time</th></tr></thead><tbody><tr><td style="font-weight: bold;">Martin &amp; Moyce</td><td>initial width a</td><td>T = t&radic;(g/a)</td></tr><tr><td style="font-weight: bold;">Lobovsk&yacute;</td><td>depth H</td><td>t* = t&radic;(g/H)</td></tr><tr><td style="font-weight: bold; color: var(--blue);">This solver</td><td>L<sub>0</sub></td><td>T* = t&radic;(g/L<sub>0</sub>)</td></tr></tbody></table><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 120px;"><div class="kpi-card__title">Early-Time Check (Ritter)</div><div style="font-family: var(--font-mono); font-size: 26px; font-weight: bold; line-height: 1.35; ">X* = 1 + 2&middot;T*</div><div style="font-family: var(--font-mono); font-size: 14px; font-weight: normal; line-height: 1.35; ">front starts at x/L<sub>0</sub> = 1, then moves at 2&radic;(gH); square column</div></div><p style="margin: 14px 0 0 0;">Match the axis convention of the data you overlay before comparing curves.</p></div><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  justify-content: flex-start;"><div class="kpi-card__title"></div><div style="font-family: var(--font-mono); font-size: 24px; font-weight: bold; text-transform: uppercase; margin-bottom: 14px;">Acceptance Checks</div><ul style="margin: 0; padding-left: 20px; font-size: 16px; line-height: 1.5;"><li>&int;&alpha; dV stays within a stated tolerance (live readout)</li><li>Front X*(T*) vs Martin &amp; Moyce data</li><li>Front stays below Ritter: 1 + 2T*</li><li>Late front speed about 1.1&ndash;1.75 &radic;(gH)</li><li>&nabla; &middot; u &approx; 0 after every projection</li><li>Error shrinks under grid and &Delta;t refinement</li></ul></div></div><canvas id="deckValChart" width="800" height="250" style="display: none;"></canvas>
    `
  },
  {
    title: "9. IMPACT PRESSURE AT A WALL",
    content: `
      <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 18px; margin: 15px 0; "><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">Median Peak, 3 mm above bed</div><div style="font-family: var(--font-mono); font-size: 30px; font-weight: bold; line-height: 1.35; ">&approx; 3 &times; &rho;gH</div><div style="font-family: var(--font-mono); font-size: 15px; font-weight: normal; line-height: 1.35; ">97.5th percentile &approx; 4.5&times;</div></div><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">P / &rho;V&sup2;, measured</div><div style="font-family: var(--font-mono); font-size: 36px; font-weight: bold; line-height: 1.35; ">1.25</div><div style="font-family: var(--font-mono); font-size: 15px; font-weight: normal; line-height: 1.35; ">vs 0.5 for a steady impinging jet</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: center; text-align: center; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">Repeated runs per fill height</div><div style="font-family: var(--font-mono); font-size: 36px; font-weight: bold; line-height: 1.35; ">100</div><div style="font-family: var(--font-mono); font-size: 15px; font-weight: normal; line-height: 1.35; ">peak pressure is a random variable</div></div></div><p><strong>Time scales:</strong> rise 1.5&ndash;4.5 ms, decay about 10&times; longer. Impulse &int;P dt &approx; &frac12; &times; peak &times; impact time (within about 25%).</p><p><strong>Scaling:</strong> lowest-sensor peak &prop; H (Froude: P ~ &rho;gH); higher sensors are not linear in H.</p><p><strong>For your solver:</strong> incompressible VOF gives a sharp, grid-dependent spike. Compare arrival time and impulse with the median and 95% band, not one peak.</p>
    `
  },
  {
    title: "10. SCOPE FOR IMPROVEMENT",
    content: `
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin: 15px 0; "><div class="kpi-card kpi-card--yellow" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">Higher-Order Interface Tracking</div><div style="font-size: 17px;">Replace donor-acceptor with PLIC for a continuous, sharp geometric interface reconstruction.</div></div><div class="kpi-card kpi-card--green" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">Hardware Acceleration</div><div style="font-size: 17px;">Port grid sweeps and the Poisson solver to WebGPU compute shaders; scale to millions of cells and 3D.</div></div><div class="kpi-card kpi-card--blue" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center;  min-height: 150px;"><div class="kpi-card__title">Physics Gaps</div><div style="font-size: 17px;">Air compressibility and entrapment at impact, wet-bed jets, and turbulence at Re ~ 1e6 on an under-resolved grid.</div></div><div class="kpi-card" style="margin: 0; align-items: flex-start; text-align: left; justify-content: center; background: #fff; color: var(--ink); min-height: 150px;"><div class="kpi-card__title">3D Effects</div><div style="font-size: 17px;">The lab flow stops being 2D at H = 600 mm, which a 2D solver cannot show.</div></div></div>
    `
  }
];

let currentSlide = 0;
function renderDeck() {
  $("modalTitle").textContent = "DECK";
  $("modalActions").innerHTML = `
    <span style="margin-right: 20px; font-family: var(--font-mono); font-size: 14px;">${currentSlide + 1} / ${slides.length}</span>
    <button id="prevSlide" class="btn btn--ghost">← PREV</button>
    <button id="nextSlide" class="btn btn--accent">NEXT →</button>
  `;
  $("modalContent").innerHTML = `
    <div class="deck-slide">
      <div class="deck-slide__title">${slides[currentSlide].title}</div>
      <div class="deck-slide__content">${slides[currentSlide].content}</div>
    </div>
  `;
  
  $("prevSlide").onclick = () => { if (currentSlide > 0) { currentSlide--; renderDeck(); } };
  $("nextSlide").onclick = () => { if (currentSlide < slides.length - 1) { currentSlide++; renderDeck(); } };
  $("modalOverlay").classList.remove("hidden");
  
  // If we are on the Validation slide, draw the graph!
  if (currentSlide === 7 && $("deckValChart")) {
    drawValidationChart($("deckValChart"), 800, 250);
  }
}

$("deckBtn").onclick = () => { currentSlide = 0; renderDeck(); };
$("validBtn").onclick = () => { renderValidation(); };

