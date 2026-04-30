"use strict";

// ---------------------------------------------------------------------------
// Constants matching physics/simulation.go
// ---------------------------------------------------------------------------
const SIM_BOX          = 800;   // matches physics.DefaultConfig().BoxSize
const DT               = 0.016; // fixed timestep (~60 fps); speed slider = steps per frame
const GRID_MAX_OPACITY = 0.28;  // cap for a fully dominant cell

// ---------------------------------------------------------------------------
// Canvas setup
// ---------------------------------------------------------------------------
const canvas  = document.getElementById("sim-canvas");
const ctx     = canvas.getContext("2d");
const wrap    = document.getElementById("canvas-wrap");

function resizeCanvas() {
  const w = wrap.clientWidth;
  const h = wrap.clientHeight;
  const side = Math.min(w, h) - 4;
  canvas.width  = side;
  canvas.height = side;
}
resizeCanvas();
window.addEventListener("resize", resizeCanvas);

// Map simulation coordinates (centred on 0) to canvas pixels.
function simToCanvas(x, y) {
  const scale = canvas.width / SIM_BOX;
  return [
    canvas.width  / 2 + x * scale,
    canvas.height / 2 + y * scale,
  ];
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------
let running         = true;
let stepsPerFrame   = 1;    // controlled by speed slider
let dotRadius       = 2;    // controlled by dot-size slider
let particleCount   = 1000;
let showDensityGrid = true;
let gridCells       = 20;   // N×N grid resolution
let showForceLines  = false;

// ---------------------------------------------------------------------------
// Controls
// ---------------------------------------------------------------------------
const btnPlay  = document.getElementById("btn-play");
const btnReset = document.getElementById("btn-reset");

btnPlay.addEventListener("click", () => {
  running = !running;
  btnPlay.textContent = running ? "Pause" : "Play";
  btnPlay.classList.toggle("active", running);
});

btnReset.addEventListener("click", () => {
  gravSim.init(particleCount);
  updateStats();
});

const speedSlider = document.getElementById("speed");
const speedVal    = document.getElementById("speed-val");
speedSlider.addEventListener("input", () => {
  stepsPerFrame = parseInt(speedSlider.value);
  speedVal.textContent = stepsPerFrame + "×";
});

const countSlider    = document.getElementById("count");
const countVal       = document.getElementById("count-val");
const forceWarning   = document.getElementById("force-warning");

function updateForceWarning() {
  forceWarning.style.display = (showForceLines && particleCount > 200) ? "inline" : "none";
}

countSlider.addEventListener("input", () => {
  particleCount = parseInt(countSlider.value);
  countVal.textContent = particleCount;
  gravSim.init(particleCount);
  updateStats();
  updateForceWarning();
});

const dotSlider = document.getElementById("dot-size");
const dotVal    = document.getElementById("dot-val");
dotSlider.addEventListener("input", () => {
  dotRadius = parseInt(dotSlider.value);
  dotVal.textContent = dotRadius;
});

document.getElementById("density-toggle").addEventListener("change", e => {
  showDensityGrid = e.target.checked;
});

document.getElementById("force-toggle").addEventListener("change", e => {
  showForceLines = e.target.checked;
  updateForceWarning();
});

const gridResSlider = document.getElementById("grid-res");
const gridVal       = document.getElementById("grid-val");
gridResSlider.addEventListener("input", () => {
  gridCells = parseInt(gridResSlider.value);
  gridVal.textContent = gridCells;
});

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------
const MATTER_COLOR     = "#4488ff"; // blue
const ANTIMATTER_COLOR = "#ff4444"; // red

// Bin particles into a gridCells×gridCells grid and shade each cell by which
// charge dominates. Dominance = (matter − antimatter) / total ∈ [−1, +1].
// Pure matter → blue, pure antimatter → red, equal → transparent.
function drawDensityGrid(particles) {
  const stride = 4;
  const n      = particles.length / stride;
  const cells  = gridCells;
  const size   = cells * cells;

  const matterCount = new Int32Array(size);
  const antiCount   = new Int32Array(size);

  for (let i = 0; i < n; i++) {
    const sx = particles[i * stride];
    const sy = particles[i * stride + 1];
    const gc = particles[i * stride + 2];

    // Map sim coords [-SIM_BOX/2, SIM_BOX/2) → cell index [0, cells).
    let cx = Math.floor((sx + SIM_BOX / 2) / SIM_BOX * cells);
    let cy = Math.floor((sy + SIM_BOX / 2) / SIM_BOX * cells);
    // Clamp — should rarely trigger after wrapping, but keeps indices safe.
    cx = Math.max(0, Math.min(cells - 1, cx));
    cy = Math.max(0, Math.min(cells - 1, cy));

    const idx = cy * cells + cx;
    if (gc > 0) matterCount[idx]++;
    else        antiCount[idx]++;
  }

  const cellW = canvas.width  / cells;
  const cellH = canvas.height / cells;

  for (let cy = 0; cy < cells; cy++) {
    for (let cx = 0; cx < cells; cx++) {
      const idx   = cy * cells + cx;
      const m     = matterCount[idx];
      const a     = antiCount[idx];
      const total = m + a;
      if (total === 0) continue;

      // dominance ∈ [−1, +1]: positive = matter-dominant, negative = anti-dominant.
      const dominance = (m - a) / total;
      const opacity   = Math.abs(dominance) * GRID_MAX_OPACITY;
      if (opacity < 0.01) continue; // nearly equal — leave transparent

      ctx.fillStyle = dominance > 0
        ? `rgba(68, 136, 255, ${opacity})`   // blue for matter
        : `rgba(255,  68,  68, ${opacity})`; // red for antimatter

      ctx.fillRect(cx * cellW, cy * cellH, cellW, cellH);
    }
  }
}

// Force vectors are in simulation units; convert to canvas pixels by scaling
// the magnitude before capping. Logarithmic compression keeps faint forces
// visible while preventing nearby-particle spikes from dominating the display.
const FORCE_LOG_SCALE = 600; // multiplier before log: tunes sensitivity
const FORCE_MAX_PX    = 45;  // hard cap on arrow length in canvas pixels

function drawArrowhead(x, y, angle, size) {
  ctx.beginPath();
  ctx.moveTo(x, y);
  ctx.lineTo(x - size * Math.cos(angle - Math.PI / 6), y - size * Math.sin(angle - Math.PI / 6));
  ctx.lineTo(x - size * Math.cos(angle + Math.PI / 6), y - size * Math.sin(angle + Math.PI / 6));
  ctx.closePath();
  ctx.fill();
}

function drawForceLines(particles, forces) {
  const stride = 4;
  const n      = particles.length / stride;

  ctx.save();
  ctx.lineWidth = 1;

  for (let i = 0; i < n; i++) {
    const fx = forces[i * 2];
    const fy = forces[i * 2 + 1];
    const fmag = Math.sqrt(fx * fx + fy * fy);
    if (fmag < 1e-10) continue;

    // Logarithmic magnitude → canvas pixels so wide dynamic range stays legible.
    const displayLen = Math.min(Math.log1p(fmag * FORCE_LOG_SCALE) * 10, FORCE_MAX_PX);
    if (displayLen < 1.5) continue;

    const nx = fx / fmag;
    const ny = fy / fmag;

    const [cx, cy] = simToCanvas(particles[i * stride], particles[i * stride + 1]);
    const ex = cx + nx * displayLen;
    const ey = cy + ny * displayLen;
    const gc = particles[i * stride + 2];

    const color = gc > 0 ? "rgba(68,136,255,0.75)" : "rgba(255,68,68,0.75)";
    ctx.strokeStyle = color;
    ctx.fillStyle   = color;

    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(ex, ey);
    ctx.stroke();

    const headLen = Math.min(5, displayLen * 0.35);
    drawArrowhead(ex, ey, Math.atan2(ny, nx), headLen);
  }

  ctx.restore();
}

function draw(particles, forces) {
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  if (showDensityGrid) {
    drawDensityGrid(particles);
  }

  // Faint grid lines to give a sense of scale.
  ctx.strokeStyle = "#1a1a28";
  ctx.lineWidth = 0.5;
  ctx.beginPath();
  ctx.moveTo(canvas.width / 2, 0);
  ctx.lineTo(canvas.width / 2, canvas.height);
  ctx.moveTo(0, canvas.height / 2);
  ctx.lineTo(canvas.width, canvas.height / 2);
  ctx.stroke();

  // Flat array layout: [x, y, gcharge, mass,  x, y, gcharge, mass, ...]
  const stride  = 4;
  const n       = particles.length / stride;
  // Dot radius scales with sqrt(mass): area proportional to mass, unchanged at mass=1.
  const MAX_DOT = dotRadius * 12;

  // Batch by colour to minimise fillStyle switches.
  ctx.fillStyle = MATTER_COLOR;
  ctx.beginPath();
  for (let i = 0; i < n; i++) {
    if (particles[i * stride + 2] > 0) {
      const r = Math.min(MAX_DOT, dotRadius * Math.sqrt(particles[i * stride + 3]));
      const [cx, cy] = simToCanvas(particles[i * stride], particles[i * stride + 1]);
      ctx.moveTo(cx + r, cy);
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
    }
  }
  ctx.fill();

  ctx.fillStyle = ANTIMATTER_COLOR;
  ctx.beginPath();
  for (let i = 0; i < n; i++) {
    if (particles[i * stride + 2] < 0) {
      const r = Math.min(MAX_DOT, dotRadius * Math.sqrt(particles[i * stride + 3]));
      const [cx, cy] = simToCanvas(particles[i * stride], particles[i * stride + 1]);
      ctx.moveTo(cx + r, cy);
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
    }
  }
  ctx.fill();

  if (forces) {
    drawForceLines(particles, forces);
  }
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------
const sMatter = document.getElementById("s-matter");
const sAnti   = document.getElementById("s-anti");
const sAnn    = document.getElementById("s-ann");
const sMerged = document.getElementById("s-merged");
const sSteps  = document.getElementById("s-steps");

function updateStats() {
  const st = gravSim.getStats();
  sMatter.textContent = st.matter;
  sAnti.textContent   = st.antimatter;
  sAnn.textContent    = st.annihilated;
  sMerged.textContent = st.merged;
  sSteps.textContent  = st.steps;
}

// ---------------------------------------------------------------------------
// Animation loop
// ---------------------------------------------------------------------------
let frameCount = 0;

function loop() {
  requestAnimationFrame(loop);

  if (running) {
    for (let i = 0; i < stepsPerFrame; i++) {
      gravSim.step(DT);
    }
  }

  const particles = gravSim.getParticles();
  const forces    = showForceLines ? gravSim.getForces() : null;
  draw(particles, forces);

  // Update stats every 10 frames to avoid DOM churn.
  if (frameCount++ % 10 === 0) {
    updateStats();
  }
}

// ---------------------------------------------------------------------------
// WASM bootstrap
// ---------------------------------------------------------------------------
async function boot() {
  const go = new Go();
  const result = await WebAssembly.instantiateStreaming(fetch("sim.wasm"), go.importObject);
  go.run(result.instance);

  // gravSim is now registered as a global by the WASM module.
  gravSim.init(particleCount);
  document.getElementById("loading").style.display = "none";
  updateStats();
  loop();
}

boot().catch(err => {
  document.getElementById("loading").textContent = "Failed to load WASM: " + err;
  console.error(err);
});
