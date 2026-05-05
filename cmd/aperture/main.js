"use strict";

// ---------------------------------------------------------------------------
// Sweep parameters — GC approved 2026-05-03
// 4 strengths × 3 falloffs × on/off = 24 runs
// ---------------------------------------------------------------------------
const STRENGTH_VALS = [0.1, 1, 10, 100];
const FALLOFF_VALS  = [0.5, 1.0, 2.0];

// HSL color: falloff → hue family, strength index → lightness
// On runs are lighter than off runs.
function configColor(si, fi, on) {
  const hues  = [210, 140, 30]; // blue, green, orange per falloff
  const light = on ? 55 + si * 8 : 28 + si * 8;
  return `hsl(${hues[fi]},75%,${light}%)`;
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------
let results = [];
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
const btnClear   = document.getElementById("btn-clear");
const btnExport  = document.getElementById("btn-export");
const btnSavePic = document.getElementById("btn-save-pic");
const statusMsg  = document.getElementById("status-msg");

function getConfig(overrides) {
  const base = {
    stepsPerShot:      parseInt(document.getElementById("inp-steps").value),
    numShots:          parseInt(document.getElementById("inp-shots").value),
    apertureHalfW:     parseFloat(document.getElementById("inp-hw").value),
    wallX:             parseFloat(document.getElementById("inp-wallx").value),
    gunInitVelY:       parseFloat(document.getElementById("inp-vel").value),
    anchorMass:        parseFloat(document.getElementById("inp-amass").value),
    anchorDistY:       parseFloat(document.getElementById("inp-adist").value),
    emitterOn:         document.getElementById("inp-emitter-on").checked,
    emitterYs:         null, // server fills from apertureHalfW when null
    emissionStrength:  parseFloat(document.getElementById("inp-strength").value),
    falloffRate:       parseFloat(document.getElementById("inp-falloff").value),
    emitterAnchorDist: parseFloat(document.getElementById("inp-eadist").value),
    emitterAnchorMass: parseFloat(document.getElementById("inp-eamass").value),
  };
  return { ...base, ...overrides };
}

function setStatus(msg) { statusMsg.textContent = msg; }

function setBusy(b) {
  busy = b;
  btnSingle.disabled = b;
  btnSweep.disabled  = b;
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

// ---------------------------------------------------------------------------
// Button handlers
// ---------------------------------------------------------------------------
btnSingle.addEventListener("click", async () => {
  if (busy) return;
  setBusy(true);
  setStatus("Running…");
  try {
    const result = await runOne(getConfig());
    const on  = result.config.emitterOn;
    const si  = STRENGTH_VALS.indexOf(result.config.emissionStrength);
    const fi  = FALLOFF_VALS.indexOf(result.config.falloffRate);
    const col = (si >= 0 && fi >= 0) ? configColor(si, fi, on) : (on ? "#bbbbff" : "#6666cc");
    addResult(result, col);
    setStatus(`Done — ${result.hitCount} hits (${(result.hitRate*100).toFixed(1)}%) kRatio=${result.kRatio.toFixed(2)} aEff=${result.aEff.toFixed(2)}`);
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
  let n = 0;
  const total = STRENGTH_VALS.length * FALLOFF_VALS.length * 2;

  for (let fi = 0; fi < FALLOFF_VALS.length; fi++) {
    for (let si = 0; si < STRENGTH_VALS.length; si++) {
      for (const on of [false, true]) {
        n++;
        const s = STRENGTH_VALS[si];
        const f = FALLOFF_VALS[fi];
        setStatus(`Sweep ${n}/${total} — strength=${s} falloff=${f} emitter=${on ? "ON" : "OFF"}…`);
        try {
          const cfg = { ...base, emissionStrength: s, falloffRate: f, emitterOn: on };
          const result = await runOne(cfg);
          addResult(result, configColor(si, fi, on));
        } catch (e) {
          setStatus("Error: " + e.message);
          setBusy(false);
          return;
        }
      }
    }
  }
  setStatus(`Sweep complete — ${total} runs`);
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
    description: "Aperture Wave Field Steering Phase A — results for GC review",
    note:        "No interpretation. Raw results per governance mandate.",
    runs:        results.map(r => r.data),
  };
  const blob = new Blob([JSON.stringify(payload)], { type: "application/json" });
  const url  = URL.createObjectURL(blob);
  const a    = document.createElement("a");
  a.href     = url;
  a.download = "aperture-phaseA-" + new Date().toISOString().slice(0,19).replace(/:/g,"-") + ".json";
  a.click();
  URL.revokeObjectURL(url);
  setStatus("Exported.");
});

btnSavePic.addEventListener("click", () => {
  if (results.length === 0) { setStatus("Nothing to capture."); return; }
  const a    = document.createElement("a");
  a.href     = canvas.toDataURL("image/png");
  a.download = "aperture-phaseA-" + new Date().toISOString().slice(0,19).replace(/:/g,"-") + ".png";
  a.click();
  setStatus("Picture saved.");
});

// ---------------------------------------------------------------------------
// Result management
// ---------------------------------------------------------------------------
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
    const d    = r.data;
    const on   = d.config.emitterOn;
    const pct  = (d.hitRate * 100).toFixed(1);
    const kr   = d.kRatio.toFixed(2);
    const ae   = d.aEff.toFixed(2);
    const s    = d.config.emissionStrength;
    const f    = d.config.falloffRate;
    return `<span class="run-tag">
      <span class="run-swatch" style="background:${r.color}"></span>
      s=${s} f=${f} ${on?"ON":"OFF"} &nbsp;${pct}% &nbsp;k×=${kr} &nbsp;Aeff=${ae}
    </span>`;
  }).join("  ·  ");
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------
const PAD = { top: 32, right: 160, bottom: 44, left: 52 };

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
  if (results.length === 0) { drawEmpty(); return; }

  const pa = plotArea();

  let yMax = 0;
  for (const r of results) {
    for (const v of r.data.binDensity) {
      if (v > yMax) yMax = v;
    }
  }
  yMax = yMax > 0 ? yMax * 1.12 : 1;

  const edges = results[0].data.binEdges;
  const bw    = edges[1] - edges[0];
  const xMin  = edges[0];
  const xMax  = edges[edges.length - 1] + bw;

  function toX(v) { return pa.x + (v - xMin) / (xMax - xMin) * pa.w; }
  function toY(v) { return pa.y + pa.h - (v / yMax) * pa.h; }

  // Grid
  ctx.strokeStyle = "#1a1a28";
  ctx.lineWidth   = 0.5;
  for (let i = 0; i <= 4; i++) {
    const y = pa.y + pa.h * (1 - i / 4);
    ctx.beginPath(); ctx.moveTo(pa.x, y); ctx.lineTo(pa.x + pa.w, y); ctx.stroke();
  }

  // Aperture center marker
  ctx.save();
  ctx.setLineDash([3, 4]);
  ctx.strokeStyle = "rgba(160,160,255,0.2)";
  ctx.lineWidth   = 1;
  const cx = toX(0);
  ctx.beginPath(); ctx.moveTo(cx, pa.y); ctx.lineTo(cx, pa.y + pa.h); ctx.stroke();
  ctx.setLineDash([]);
  ctx.restore();

  // Histogram bars
  for (const { data, color } of results) {
    const alpha = results.length === 1 ? 0.72 : 0.40;
    ctx.fillStyle = hexOrHslAlpha(color, alpha);
    for (let i = 0; i < data.binDensity.length; i++) {
      const v = data.binDensity[i];
      if (v <= 0) continue;
      const x1 = toX(data.binEdges[i]);
      const x2 = toX(data.binEdges[i] + bw);
      ctx.fillRect(x1, toY(v), x2 - x1 - 0.5, pa.y + pa.h - toY(v));
    }
  }

  drawAxes(pa, xMin, xMax, yMax, toX, toY);
  drawAnnotation(pa);
}

function drawEmpty() {
  const pa = plotArea();
  ctx.fillStyle    = "#202030";
  ctx.font         = "12px monospace";
  ctx.textAlign    = "center";
  ctx.textBaseline = "middle";
  ctx.fillText("Run a simulation to see the aperture distribution.", pa.x + pa.w / 2, pa.y + pa.h / 2);
  drawAxes(pa, -2.5, 2.5, 1, v => PAD.left + (v + 2.5) / 5 * pa.w, v => PAD.top + pa.h - v * pa.h);
}

function drawAxes(pa, xMin, xMax, yMax, toX, toY) {
  ctx.strokeStyle = "#3a3a60";
  ctx.lineWidth   = 1;
  ctx.beginPath(); ctx.moveTo(pa.x, pa.y + pa.h); ctx.lineTo(pa.x + pa.w, pa.y + pa.h); ctx.stroke();
  ctx.beginPath(); ctx.moveTo(pa.x, pa.y);         ctx.lineTo(pa.x, pa.y + pa.h);         ctx.stroke();

  ctx.fillStyle    = "#606080";
  ctx.font         = "10px monospace";
  ctx.textAlign    = "center";
  ctx.textBaseline = "top";
  const xRange = xMax - xMin;
  const xStep  = xRange <= 6 ? 0.5 : 1.0;
  for (let v = Math.ceil(xMin / xStep) * xStep; v <= xMax; v += xStep) {
    const cx = toX(v);
    ctx.beginPath(); ctx.moveTo(cx, pa.y + pa.h); ctx.lineTo(cx, pa.y + pa.h + 3); ctx.stroke();
    ctx.fillText(v.toFixed(1), cx, pa.y + pa.h + 5);
  }

  ctx.textAlign    = "right";
  ctx.textBaseline = "middle";
  for (let i = 0; i <= 4; i++) {
    const v  = (i / 4) * yMax;
    const cy = toY(v);
    ctx.beginPath(); ctx.moveTo(pa.x - 3, cy); ctx.lineTo(pa.x, cy); ctx.stroke();
    ctx.fillText(v.toFixed(2), pa.x - 5, cy);
  }

  ctx.fillStyle    = "#808090";
  ctx.font         = "11px monospace";
  ctx.textAlign    = "center";
  ctx.textBaseline = "bottom";
  ctx.fillText("gun y-position at shot", pa.x + pa.w / 2, canvas.height - 2);
  ctx.save();
  ctx.translate(12, pa.y + pa.h / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.textBaseline = "middle";
  ctx.fillText("density", 0, 0);
  ctx.restore();
}

// Pre-run derivation annotation box (top-right of plot area)
function drawAnnotation(pa) {
  if (results.length === 0) return;
  const d0 = results[0].data;

  const lines = [
    "Pre-run derivation",
    `ω_gun  = 0.272 rad/t`,
    `T_gun  = ${d0.oscPeriod.toFixed(0)} steps`,
    `A_null ≈ 18.4 units`,
    `sMin(f=1) = 1566`,
    `emitAmp = 3.8e-5 units`,
    "",
    "Newton 3rd asymmetry:",
    "acknowledged (phenom model)",
  ];

  const bx = pa.x + pa.w + 8;
  const by = pa.y;
  const lineH = 14;
  const boxW  = PAD.right - 12;
  const boxH  = lines.length * lineH + 10;

  ctx.fillStyle = "rgba(12,12,24,0.85)";
  ctx.fillRect(bx, by, boxW, boxH);
  ctx.strokeStyle = "#2a2a44";
  ctx.lineWidth   = 0.5;
  ctx.strokeRect(bx, by, boxW, boxH);

  ctx.font         = "9px monospace";
  ctx.textAlign    = "left";
  ctx.textBaseline = "top";
  lines.forEach((line, i) => {
    ctx.fillStyle = i === 0 ? "#8080c0" : (line.startsWith("Newton") || line.startsWith("acknowledged") ? "#504060" : "#505070");
    ctx.fillText(line, bx + 5, by + 5 + i * lineH);
  });
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------
function hexOrHslAlpha(color, alpha) {
  if (color.startsWith("hsl")) {
    return color.replace("hsl(", `hsla(`).replace(")", `,${alpha})`);
  }
  const r = parseInt(color.slice(1, 3), 16);
  const g = parseInt(color.slice(3, 5), 16);
  const b = parseInt(color.slice(5, 7), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}
