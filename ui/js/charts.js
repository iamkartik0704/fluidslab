// DAMBREAK — minimal Canvas2D chart helper (no libraries).
// Neo-brutalist: 3px ink axes, mono tick labels, hard square markers.

const INK = "#0A0A0A";
const MONO = '"Space Mono", ui-monospace, Consolas, monospace';

/** Nice tick generation: returns ~count "nice" steps covering [min,max]. */
export function niceTicks(min, max, count = 5) {
  if (!isFinite(min) || !isFinite(max) || min === max) {
    return [min || 0, (min || 0) + 1];
  }
  const span = max - min;
  const raw = span / Math.max(1, count);
  const mag = Math.pow(10, Math.floor(Math.log10(raw)));
  const norm = raw / mag;
  let step;
  if (norm < 1.5) step = 1;
  else if (norm < 3) step = 2;
  else if (norm < 7) step = 5;
  else step = 10;
  step *= mag;
  const start = Math.ceil(min / step) * step;
  const ticks = [];
  for (let v = start; v <= max + step * 1e-9; v += step) ticks.push(+v.toFixed(12));
  return ticks;
}

function fmtTick(v) {
  if (v === 0) return "0";
  const a = Math.abs(v);
  if (a >= 1e4 || a < 1e-3) return v.toExponential(0).replace("e+", "e");
  return +v.toFixed(4) + "";
}

/**
 * createChart(canvas, opts) -> {update, destroy}
 * opts: { type:'line'|'area'|'scatter', xLabel, yLabel, yRange:[min,max],
 *         padding, onHover }
 * series item: { name, color, dash, width, points:[{x,y}], marker:'square'|'circle' }
 */
export function createChart(canvas, opts = {}) {
  const ctx = canvas.getContext("2d");
  let series = [];
  let hover = null;          // {si, pi, x, y}
  let xMin = 0, xMax = 1, yMin = 0, yMax = 1;
  let cssW = 0, cssH = 0;

  const pad = Object.assign({ l: 56, r: 14, t: 14, b: 34 }, opts.padding || {});

  // --- DPR-aware sizing ------------------------------------------------------
  function resize() {
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.getBoundingClientRect();
    cssW = Math.max(80, rect.width);
    cssH = Math.max(60, rect.height);
    canvas.width = Math.round(cssW * dpr);
    canvas.height = Math.round(cssH * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw();
  }

  // --- scales ----------------------------------------------------------------
  function computeRanges() {
    let xmin = Infinity, xmax = -Infinity, ymin = Infinity, ymax = -Infinity;
    for (const s of series) {
      for (const p of s.points) {
        if (!isFinite(p.x) || !isFinite(p.y)) continue;
        if (p.x < xmin) xmin = p.x;
        if (p.x > xmax) xmax = p.x;
        if (p.y < ymin) ymin = p.y;
        if (p.y > ymax) ymax = p.y;
      }
    }
    if (!isFinite(xmin)) { xmin = 0; xmax = 1; }
    if (!isFinite(ymin)) { ymin = 0; ymax = 1; }
    if (xmin === xmax) { xmin -= 0.5; xmax += 0.5; }
    if (ymin === ymax) { ymin -= 0.5; ymax += 0.5; }
    // y padding (5% each side)
    const ypad = (ymax - ymin) * 0.05;
    ymin -= ypad; ymax += ypad;
    if (opts.yRange) {
      if (opts.yRange[0] !== undefined) ymin = opts.yRange[0];
      if (opts.yRange[1] !== undefined) ymax = opts.yRange[1];
    }
    xMin = xmin; xMax = xmax; yMin = ymin; yMax = ymax;
  }

  const X = (x) => pad.l + (x - xMin) / (xMax - xMin) * (cssW - pad.l - pad.r);
  const Y = (y) => cssH - pad.b - (y - yMin) / (yMax - yMin) * (cssH - pad.t - pad.b);

  // --- drawing ---------------------------------------------------------------
  function draw() {
    ctx.clearRect(0, 0, cssW, cssH);
    ctx.fillStyle = "#FFFFFF";
    ctx.fillRect(0, 0, cssW, cssH);

    // Horizontal gridlines (1px, faint).
    const yTicks = niceTicks(yMin, yMax, 4);
    ctx.strokeStyle = "rgba(0,0,0,.12)";
    ctx.lineWidth = 1;
    for (const t of yTicks) {
      const y = Y(t);
      ctx.beginPath();
      ctx.moveTo(pad.l, y);
      ctx.lineTo(cssW - pad.r, y);
      ctx.stroke();
    }

    // Axes: 3px ink.
    ctx.strokeStyle = INK;
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(pad.l, pad.t);
    ctx.lineTo(pad.l, cssH - pad.b);       // y axis
    ctx.lineTo(cssW - pad.r, cssH - pad.b); // x axis
    ctx.stroke();

    // Tick labels (mono 11px).
    ctx.fillStyle = "#555555";
    ctx.font = `700 11px ${MONO}`;
    ctx.textAlign = "right";
    ctx.textBaseline = "middle";
    for (const t of yTicks) ctx.fillText(fmtTick(t), pad.l - 8, Y(t));
    ctx.textAlign = "center";
    ctx.textBaseline = "top";
    const xTicks = niceTicks(xMin, xMax, 6);
    for (const t of xTicks) ctx.fillText(fmtTick(t), X(t), cssH - pad.b + 8);

    // Axis titles.
    if (opts.xLabel) {
      ctx.fillText(opts.xLabel, (pad.l + cssW - pad.r) / 2, cssH - pad.b + 22);
    }
    if (opts.yLabel) {
      ctx.save();
      ctx.translate(12, (pad.t + cssH - pad.b) / 2);
      ctx.rotate(-Math.PI / 2);
      ctx.textBaseline = "middle";
      ctx.fillText(opts.yLabel, 0, 0);
      ctx.restore();
    }

    // Series.
    for (let si = 0; si < series.length; si++) {
      const s = series[si];
      const pts = s.points.filter((p) => isFinite(p.x) && isFinite(p.y));
      if (!pts.length) continue;

      // Area fill under the line.
      if (s.area || (opts.type === "area" && !s.line === false)) {
        ctx.beginPath();
        ctx.moveTo(X(pts[0].x), Y(Math.max(yMin, Math.min(yMax, pts[0].y))));
        for (const p of pts.slice(1)) ctx.lineTo(X(p.x), Y(p.y));
        ctx.lineTo(X(pts[pts.length - 1].x), Y(Math.max(yMin, Math.min(yMax, pts[0].y)) * 0 + yMin));
        ctx.closePath();
        ctx.fillStyle = s.fillColor || "#D9DDFB";
        ctx.fill();
      }

      // Line.
      if (s.line !== false) {
        ctx.strokeStyle = s.color || INK;
        ctx.lineWidth = s.width || 3;
        ctx.setLineDash(s.dash ? [8, 6] : []);
        ctx.lineJoin = "round";
        ctx.lineCap = "round";
        ctx.beginPath();
        pts.forEach((p, i) => {
          const px = X(p.x), py = Y(p.y);
          if (i === 0) ctx.moveTo(px, py);
          else ctx.lineTo(px, py);
        });
        ctx.stroke();
        ctx.setLineDash([]);
      }

      // Point markers (squares with 2px ink outline).
      const showPts = s.points !== false && (opts.type === "scatter" || s.markers);
      if (showPts || pts.length <= 24) {
        for (const p of pts) {
          drawMarker(X(p.x), Y(p.y), s.color || INK, s.marker || "square");
        }
      }
    }

    // Hover highlight + guide.
    if (hover) {
      const s = series[hover.si];
      const p = s && s.points[hover.pi];
      if (p) {
        const px = X(p.x), py = Y(p.y);
        ctx.strokeStyle = "rgba(0,0,0,.35)";
        ctx.lineWidth = 1;
        ctx.setLineDash([4, 4]);
        ctx.beginPath();
        ctx.moveTo(px, pad.t);
        ctx.lineTo(px, cssH - pad.b);
        ctx.stroke();
        ctx.setLineDash([]);
        drawMarker(px, py, hover.si % 2 ? "#FF4A1C" : "#FFD600", "circle", 7);
      }
    }
  }

  function drawMarker(x, y, color, shape, r = 4.5) {
    ctx.fillStyle = color;
    ctx.strokeStyle = INK;
    ctx.lineWidth = 2;
    if (shape === "circle") {
      ctx.beginPath();
      ctx.arc(x, y, r, 0, 2 * Math.PI);
      ctx.fill();
      ctx.stroke();
    } else {
      ctx.fillRect(x - r, y - r, 2 * r, 2 * r);
      ctx.strokeRect(x - r, y - r, 2 * r, 2 * r);
    }
  }

  // --- hover ------------------------------------------------------------------
  function onMove(ev) {
    if (!series.length) return;
    const rect = canvas.getBoundingClientRect();
    const mx = ev.clientX - rect.left;
    const my = ev.clientY - rect.top;
    if (mx < pad.l - 6 || mx > cssW - pad.r + 6) return setHover(null);
    let best = null;
    for (let si = 0; si < series.length; si++) {
      const pts = series[si].points;
      for (let pi = 0; pi < pts.length; pi++) {
        const p = pts[pi];
        if (!isFinite(p.x) || !isFinite(p.y)) continue;
        const dx = Math.abs(X(p.x) - mx);
        if (!best || dx < best.dx) best = { si, pi, dx, x: X(p.x), y: Y(p.y) };
      }
    }
    setHover(best && best.dx < 40 ? best : null);
  }
  function setHover(h) {
    hover = h;
    draw();
    if (opts.onHover) {
      if (h) {
        const s = series[h.si];
        const p = s.points[h.pi];
        opts.onHover({ series: s.name, x: p.x, y: p.y, px: h.x, py: h.y });
      } else opts.onHover(null);
    }
  }
  canvas.addEventListener("mousemove", onMove);
  canvas.addEventListener("mouseleave", () => setHover(null));

  const ro = new ResizeObserver(() => resize());
  ro.observe(canvas);
  resize();

  return {
    update(next) {
      series = next || [];
      computeRanges();
      draw();
    },
    destroy() {
      ro.disconnect();
      canvas.removeEventListener("mousemove", onMove);
    },
  };
}
