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
let particleCount   = 500;
let showDensityGrid = true;
let gridCells       = 20;   // N×N grid resolution

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

const countSlider = document.getElementById("count");
const countVal    = document.getElementById("count-val");
countSlider.addEventListener("input", () => {
  particleCount = parseInt(countSlider.value);
  countVal.textContent = particleCount;
  gravSim.init(particleCount);
  updateStats();
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
  const stride = 3;
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

function draw(particles) {
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

  // Flat array layout: [x, y, gcharge,  x, y, gcharge, ...]
  const stride = 3;
  const n      = particles.length / stride;

  // Batch by color to minimise fillStyle switches.
  ctx.fillStyle = MATTER_COLOR;
  ctx.beginPath();
  for (let i = 0; i < n; i++) {
    if (particles[i * stride + 2] > 0) { // gcharge > 0 → matter
      const [cx, cy] = simToCanvas(particles[i * stride], particles[i * stride + 1]);
      ctx.moveTo(cx + dotRadius, cy);
      ctx.arc(cx, cy, dotRadius, 0, Math.PI * 2);
    }
  }
  ctx.fill();

  ctx.fillStyle = ANTIMATTER_COLOR;
  ctx.beginPath();
  for (let i = 0; i < n; i++) {
    if (particles[i * stride + 2] < 0) { // gcharge < 0 → antimatter
      const [cx, cy] = simToCanvas(particles[i * stride], particles[i * stride + 1]);
      ctx.moveTo(cx + dotRadius, cy);
      ctx.arc(cx, cy, dotRadius, 0, Math.PI * 2);
    }
  }
  ctx.fill();
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------
const sMatter = document.getElementById("s-matter");
const sAnti   = document.getElementById("s-anti");
const sAnn    = document.getElementById("s-ann");
const sSteps  = document.getElementById("s-steps");

function updateStats() {
  const st = gravSim.getStats();
  sMatter.textContent = st.matter;
  sAnti.textContent   = st.antimatter;
  sAnn.textContent    = st.annihilated;
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
  draw(particles);

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
