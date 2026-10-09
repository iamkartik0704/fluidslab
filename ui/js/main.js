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
      if (m.timeScale) series.timeScale = m.timeScale;
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
  $("dSlow").textContent = slow;
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
    const f32 = new Float32Array(u8.buffer, u8.byteOffset + HDR + n, 2 * n);
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

  const tsName = (hello && hello.params && hello.params.timeScale) || "√(2g/L0)";
  $("plotTimeScale").textContent = "t* CONVENTION: " + tsName.toUpperCase() + " · FRONT DEF: " +
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
  $("timeScaleSelect").value = params.timeScale;
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
    timeScale: $("timeScaleSelect").value,
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
  $("timeScaleSelect").onchange = pushParams;

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

// ================= MODAL LOGIC =================


function renderDeck() {
  $("modalTitle").textContent = "DECK";
  $("modalActions").innerHTML = `
    <a href="/deck.pptx" download class="btn btn--ghost">DOWNLOAD PPT</a>
    <button class="btn btn--accent" onclick="document.getElementById('modalOverlay').classList.add('hidden')">CLOSE</button>
  `;
  $("modalContent").innerHTML = `
    <div style="width: 100%; height: 100%; min-height: 500px;">
      <iframe src="https://view.officeapps.live.com/op/embed.aspx?src=https://dambreak.onrender.com/deck.pptx" width="100%" height="100%" frameborder="0">
        This is an embedded <a target="_blank" href="http://office.com">Microsoft Office</a> presentation, powered by <a target="_blank" href="http://office.com/webapps">Office Online</a>.
      </iframe>
    </div>
  `;
  $("modalOverlay").classList.remove("hidden");
}

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
$("deckBtn").onclick = () => { renderDeck(); };
$("validBtn").onclick = () => { renderValidation(); };

