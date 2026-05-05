"use strict";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------
const SWEEP_RATES = [1, 5, 10, 25, 50, 100, 250, 500];

// One color per breathing rate (cold→warm: slow→fast oscillation sampling).
const RATE_COLORS = [
  "#4488ff", // 1
  "#44aaee", // 5
  "#44ccbb", // 10
  "#44cc77", // 25
  "#99cc44", // 50
  "#ccaa33", // 100
  "#cc6633", // 250
  "#ff4444", // 500
];

// Emitter-on variants: lighter/brighter version of each rate color.
const ON_COLORS = [
  "#aaccff", // 1
  "#aadeff", // 5
  "#aaffee", // 10
  "#aaffcc", // 25
  "#ddffaa", // 50
  "#ffee99", // 100
  "#ffbb99", // 250
  "#ff9999", // 500
];

let results = []; // accumulated RunResult objects
let busy    = false;

// ---------------------------------------------------------------------------
// Canvas
// ---------------------------------------------------------------------------
const canvas = document.getElementById("hist-canvas");
const ctx    = canvas.getContext("2d");
const wrap   = document.getElementById("canvas-wrap");

function resizeCanvas() {
  canvas.width  = wrap.clientWidth  - 24;
  canvas.height = wrap.clientHeight - 24;
  render();
}
resizeCanvas();
window.addEventListener("resize", resizeCanvas);

// ---------------------------------------------------------------------------
// Controls
// ---------------------------------------------------------------------------
const btnSingle  = document.getElementById("btn-single");
const btnSweep   = document.getElementById("btn-sweep");
const btnOnOff   = document.getElementById("btn-onoff");
const btnClear   = document.getElementById("btn-clear");
const btnExport  = document.getElementById("btn-export");
const btnSavePic = document.getElementById("btn-save-pic");
const statusMsg = document.getElementById("status-msg");

function getConfig(stepsPerShot, emitterOn) {
  return {
    stepsPerShot:      stepsPerShot ?? parseInt(document.getElementById("inp-steps").value),
    numShots:          parseInt(document.getElementById("inp-shots").value),
    slitSepY:          parseFloat(document.getElementById("inp-sep").value),
    slitWidth:         parseFloat(document.getElementById("inp-width").value),
    gunInitVelY:       parseFloat(document.getElementById("inp-vel").value),
    anchorMass:        parseFloat(document.getElementById("inp-amass").value),
    anchorDistY:       parseFloat(document.getElementById("inp-adist").value),
    emitterOn:         emitterOn ?? false,
    emitterWallX:      parseFloat(document.getElementById("inp-wallx").value),
    emitterMass:       parseFloat(document.getElementById("inp-emass").value),
    emitterAnchorDist: parseFloat(document.getElementById("inp-eadist").value),
    emitterAnchorMass: parseFloat(document.getElementById("inp-eamass").value),
  };
}

function setStatus(msg) { statusMsg.textContent = msg; }

function setBusy(b) {
  busy = b;
  btnSingle.disabled = b;
  btnSweep.disabled  = b;
  btnOnOff.disabled  = b;
}

async function runOne(cfg) {
  const resp = await fetch("/run", {
    method:  "POST",
    headers: { "Content-Type": "application/json" },
    body:    JSON.stringify(cfg),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

btnSingle.addEventListener("click", async () => {
  if (busy) return;
  setBusy(true);
  setStatus("Running…");
  try {
    const result = await runOne(getConfig());
    addResult(result, colorForRate(result.config.stepsPerShot));
    setStatus(`Done — ${result.hitCount} hits / ${result.config.numShots} shots (${(result.hitRate * 100).toFixed(1)}% hit rate)`);
  } catch (e) {
    setStatus("Error: " + e.message);
  }
  setBusy(false);
});

btnSweep.addEventListener("click", async () => {
  if (busy) return;
  setBusy(true);
  results = [];
  const base = getConfig();
  for (let i = 0; i < SWEEP_RATES.length; i++) {
    const rate = SWEEP_RATES[i];
    setStatus(`Sweep ${i + 1}/${SWEEP_RATES.length} — rate=${rate} steps/shot…`);
    try {
      const result = await runOne({ ...base, stepsPerShot: rate });
      addResult(result, RATE_COLORS[i]);
    } catch (e) {
      setStatus("Error at rate " + rate + ": " + e.message);
      setBusy(false);
      return;
    }
  }
  setStatus(`Sweep complete — ${SWEEP_RATES.length} runs documented`);
  setBusy(false);
});

btnOnOff.addEventListener("click", async () => {
  if (busy) return;
  setBusy(true);
  results = [];
  const base = getConfig();
  for (let i = 0; i < SWEEP_RATES.length; i++) {
    const rate = SWEEP_RATES[i];
    setStatus(`On/off sweep ${i * 2 + 1}/16 — rate=${rate} emitter OFF…`);
    try {
      const off = await runOne({ ...base, stepsPerShot: rate, emitterOn: false });
      addResult(off, RATE_COLORS[i]);
    } catch (e) {
      setStatus("Error: " + e.message); setBusy(false); return;
    }
    setStatus(`On/off sweep ${i * 2 + 2}/16 — rate=${rate} emitter ON…`);
    try {
      const on = await runOne({ ...base, stepsPerShot: rate, emitterOn: true });
      addResult(on, ON_COLORS[i]);
    } catch (e) {
      setStatus("Error: " + e.message); setBusy(false); return;
    }
  }
  const maxEmit = Math.max(...results.filter(r => r.data.config.emitterOn).map(r => r.data.emitAmp));
  setStatus(`On/off sweep complete — 16 runs. Peak emitter displacement: ${maxEmit.toExponential(3)} units`);
  setBusy(false);
});

btnClear.addEventListener("click", () => {
  results = [];
  render();
  updateResultsBar();
  setStatus("Cleared");
});

btnExport.addEventListener("click", () => {
  if (results.length === 0) { setStatus("Nothing to export."); return; }
  const payload = {
    exportedAt:  new Date().toISOString(),
    description: "DSlit breathing apparatus — results for GC review",
    note:        "No interpretation. Distribution documented per governance mandate.",
    runs:        results.map(r => r.data),
  };
  const blob = new Blob([JSON.stringify(payload)], { type: "application/json" });
  const url  = URL.createObjectURL(blob);
  const a    = document.createElement("a");
  a.href     = url;
  a.download = "dslit-results-" + new Date().toISOString().slice(0, 19).replace(/:/g, "-") + ".json";
  a.click();
  URL.revokeObjectURL(url);
  setStatus("Exported.");
});

btnSavePic.addEventListener("click", () => {
  if (results.length === 0) { setStatus("Nothing to capture."); return; }
  const a    = document.createElement("a");
  a.href     = canvas.toDataURL("image/png");
  a.download = "dslit-" + new Date().toISOString().slice(0, 19).replace(/:/g, "-") + ".png";
  a.click();
  setStatus("Picture saved.");
});

// ---------------------------------------------------------------------------
// Result management
// ---------------------------------------------------------------------------
function colorForRate(stepsPerShot) {
  const idx = SWEEP_RATES.indexOf(stepsPerShot);
  return idx >= 0 ? RATE_COLORS[idx] : "#9090c0";
}

function addResult(data, color) {
  results.push({ data, color });
  render();
  updateResultsBar();
}

function updateResultsBar() {
  const bar = document.getElementById("results-bar");
  if (results.length === 0) {
    bar.innerHTML = '<span id="results-summary">No runs yet.</span>';
    return;
  }
  bar.innerHTML = results.map(r => {
    const d      = r.data;
    const hitPct = (d.hitRate * 100).toFixed(1);
    const state  = d.config.emitterOn ? "ON" : "OFF";
    const emitInfo = d.config.emitterOn ? ` emitAmp=${d.emitAmp.toExponential(2)}` : "";
    return `<span class="run-tag">
      <span class="run-swatch" style="background:${r.color}"></span>
      rate=${d.config.stepsPerShot} ${state} &nbsp; ${hitPct}%${emitInfo}
    </span>`;
  }).join("  ·  ");
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------
const PAD = { top: 24, right: 20, bottom: 44, left: 52 };

function plotArea() {
  return {
    x: PAD.left,
    y: PAD.top,
    w: canvas.width  - PAD.left - PAD.right,
    h: canvas.height - PAD.top  - PAD.bottom,
  };
}

function render() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  if (results.length === 0) {
    drawEmpty();
    return;
  }

  const pa = plotArea();

  // Find y-axis max across all density arrays
  let yMax = 0;
  for (const r of results) {
    for (const v of r.data.binDensity) {
      if (v > yMax) yMax = v;
    }
  }
  yMax = yMax > 0 ? yMax * 1.12 : 1;

  const edges = results[0].data.binEdges;
  const binW  = edges[1] - edges[0];
  const xMin  = edges[0];
  const xMax  = edges[edges.length - 1] + binW;

  function toCanvasX(v) { return pa.x + (v - xMin) / (xMax - xMin) * pa.w; }
  function toCanvasY(v) { return pa.y + pa.h - (v / yMax) * pa.h; }

  // Grid lines
  ctx.strokeStyle = "#1a1a28";
  ctx.lineWidth   = 0.5;
  const yTicks = 4;
  for (let i = 0; i <= yTicks; i++) {
    const y = pa.y + pa.h * (1 - i / yTicks);
    ctx.beginPath();
    ctx.moveTo(pa.x, y);
    ctx.lineTo(pa.x + pa.w, y);
    ctx.stroke();
  }

  // Slit position markers — use config from first result
  const cfg  = results[0].data.config;
  const slit1 = cfg.slitSepY / 2;
  const slit2 = -cfg.slitSepY / 2;
  const hw    = cfg.slitWidth / 2;

  ctx.save();
  ctx.setLineDash([3, 4]);
  ctx.strokeStyle = "rgba(160, 160, 255, 0.25)";
  ctx.lineWidth   = 1;
  for (const sc of [slit1, slit2]) {
    const cx = toCanvasX(sc);
    ctx.beginPath();
    ctx.moveTo(cx, pa.y);
    ctx.lineTo(cx, pa.y + pa.h);
    ctx.stroke();
    // Slit extent shading
    const x1 = toCanvasX(sc - hw);
    const x2 = toCanvasX(sc + hw);
    ctx.fillStyle = "rgba(80, 80, 160, 0.06)";
    ctx.fillRect(x1, pa.y, x2 - x1, pa.h);
  }
  ctx.setLineDash([]);
  ctx.restore();

  // Histogram bars (back to front: earlier runs first, lower opacity)
  for (let ri = 0; ri < results.length; ri++) {
    const { data, color } = results[ri];
    const alpha = results.length === 1 ? 0.72 : 0.45;
    ctx.fillStyle = color.replace(")", `, ${alpha})`).replace("rgb(", "rgba(").replace("#", "");
    // Use hex-to-rgba helper
    ctx.fillStyle = hexAlpha(color, alpha);

    for (let i = 0; i < data.binDensity.length; i++) {
      const v  = data.binDensity[i];
      if (v <= 0) continue;
      const x1 = toCanvasX(data.binEdges[i]);
      const x2 = toCanvasX(data.binEdges[i] + binW);
      const y1 = toCanvasY(v);
      const y2 = pa.y + pa.h;
      ctx.fillRect(x1, y1, x2 - x1 - 1, y2 - y1);
    }
  }

  // Classical expected overlay (arcsine distribution through slits)
  // Uses config from first result, amplitude from observed oscAmp.
  const classical = computeClassical(results[0].data);
  if (classical) {
    ctx.save();
    ctx.strokeStyle = "rgba(220, 220, 255, 0.5)";
    ctx.lineWidth   = 1.5;
    ctx.setLineDash([4, 3]);
    ctx.beginPath();
    let first = true;
    for (let i = 0; i < classical.length; i++) {
      const x = toCanvasX(edges[i] + binW / 2);
      const y = toCanvasY(classical[i]);
      if (first) { ctx.moveTo(x, y); first = false; }
      else         ctx.lineTo(x, y);
    }
    ctx.stroke();
    ctx.restore();
  }

  drawAxes(pa, xMin, xMax, yMax, toCanvasX, toCanvasY);
}

function drawEmpty() {
  const pa = plotArea();
  ctx.fillStyle = "#202030";
  ctx.font = "12px monospace";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText("Run a simulation to see the screen distribution.", pa.x + pa.w / 2, pa.y + pa.h / 2);
  drawAxes(pa, -30, 30, 1, v => PAD.left + (v + 30) / 60 * pa.w, v => PAD.top + pa.h - v * pa.h);
}

function drawAxes(pa, xMin, xMax, yMax, toCanvasX, toCanvasY) {
  ctx.strokeStyle = "#3a3a60";
  ctx.lineWidth   = 1;

  // X axis
  ctx.beginPath();
  ctx.moveTo(pa.x,          pa.y + pa.h);
  ctx.lineTo(pa.x + pa.w,   pa.y + pa.h);
  ctx.stroke();

  // Y axis
  ctx.beginPath();
  ctx.moveTo(pa.x, pa.y);
  ctx.lineTo(pa.x, pa.y + pa.h);
  ctx.stroke();

  ctx.fillStyle    = "#606080";
  ctx.font         = "10px monospace";
  ctx.textAlign    = "center";
  ctx.textBaseline = "top";

  // X ticks
  const xRange = xMax - xMin;
  const xStep  = xRange <= 30 ? 5 : 10;
  for (let v = Math.ceil(xMin / xStep) * xStep; v <= xMax; v += xStep) {
    const cx = toCanvasX(v);
    ctx.beginPath();
    ctx.moveTo(cx, pa.y + pa.h);
    ctx.lineTo(cx, pa.y + pa.h + 3);
    ctx.stroke();
    ctx.fillText(v.toFixed(0), cx, pa.y + pa.h + 5);
  }

  // Y ticks
  ctx.textAlign    = "right";
  ctx.textBaseline = "middle";
  const yTicks = 4;
  for (let i = 0; i <= yTicks; i++) {
    const v  = (i / yTicks) * yMax;
    const cy = toCanvasY(v);
    ctx.beginPath();
    ctx.moveTo(pa.x - 3, cy);
    ctx.lineTo(pa.x, cy);
    ctx.stroke();
    ctx.fillText(v.toFixed(2), pa.x - 5, cy);
  }

  // Axis labels
  ctx.fillStyle    = "#808090";
  ctx.font         = "11px monospace";
  ctx.textAlign    = "center";
  ctx.textBaseline = "bottom";
  ctx.fillText("screen y-position", pa.x + pa.w / 2, canvas.height - 2);

  ctx.save();
  ctx.translate(12, pa.y + pa.h / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.textBaseline = "middle";
  ctx.fillText("density", 0, 0);
  ctx.restore();

  // Legend
  if (results.length > 0) {
    ctx.textAlign    = "left";
    ctx.textBaseline = "top";
    ctx.font         = "10px monospace";
    ctx.fillStyle    = "rgba(150,150,180,0.6)";
    ctx.fillText("--- expected (arcsine)", pa.x + 4, pa.y + 4);
  }
}

// ---------------------------------------------------------------------------
// Classical expected distribution
// ---------------------------------------------------------------------------
function computeClassical(runData) {
  const A = runData.oscAmp;
  if (A <= 0) return null;

  const edges = runData.binEdges;
  const bw    = edges[1] - edges[0];
  const n     = edges.length;
  const slit1 = runData.config.slitSepY / 2;
  const slit2 = -runData.config.slitSepY / 2;
  const hw    = runData.config.slitWidth / 2;

  const counts = new Float64Array(n);
  let total = 0;
  const samples = 200000;

  for (let i = 0; i < samples; i++) {
    const phase = (2 * Math.PI * i) / samples;
    const y = A * Math.sin(phase);

    if (Math.abs(y - slit1) <= hw || Math.abs(y - slit2) <= hw) {
      total++;
      const bin = Math.floor((y - edges[0]) / bw);
      if (bin >= 0 && bin < n) counts[bin]++;
    }
  }

  if (total === 0) return null;
  return Array.from(counts).map(c => c / (total * bw));
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------
function hexAlpha(hex, alpha) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}
