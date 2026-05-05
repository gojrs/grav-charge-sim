"use strict";
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

// ---------------------------------------------------------------------------
// Environment context
// ---------------------------------------------------------------------------
const env = await fetch('/env').then(r => r.json()).catch(() => ({ type: 'local', workers: 1 }));
document.getElementById('env-workers').textContent = env.workers;
window.GRAV_ENV = env;

// ---------------------------------------------------------------------------
// Renderer
// ---------------------------------------------------------------------------
const wrap     = document.getElementById('renderer-wrap');
const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
renderer.setClearColor(0x0a0a0f);
wrap.appendChild(renderer.domElement);
renderer.domElement.style.cssText = 'position:absolute;top:0;left:0;';

const scene  = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(55, 1, 1, 10000);
camera.position.set(0, 0, 1400);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping  = true;
controls.dampingFactor  = 0.08;
controls.screenSpacePanning = false;

function resize() {
  const w = wrap.clientWidth, h = wrap.clientHeight;
  renderer.setSize(w, h);
  camera.aspect = w / h;
  camera.updateProjectionMatrix();
}
resize();
window.addEventListener('resize', resize);

// ---------------------------------------------------------------------------
// Lighting
// ---------------------------------------------------------------------------
scene.add(new THREE.AmbientLight(0x28283a, 5));
const sun  = new THREE.DirectionalLight(0xffffff, 1.5);
sun.position.set(600, 800, 500);
scene.add(sun);
const fill = new THREE.DirectionalLight(0x404060, 0.5);
fill.position.set(-400, -300, -200);
scene.add(fill);

// ---------------------------------------------------------------------------
// Particle meshes — InstancedMesh, one per charge type
// ---------------------------------------------------------------------------
const MAX_EACH  = 6000;
const sphereGeo = new THREE.SphereGeometry(1, 10, 7);

const matterMesh = new THREE.InstancedMesh(
  sphereGeo,
  new THREE.MeshPhongMaterial({ color: 0x4488ff, emissive: 0x0a1830, shininess: 90 }),
  MAX_EACH
);
const antiMesh = new THREE.InstancedMesh(
  sphereGeo,
  new THREE.MeshPhongMaterial({ color: 0xff4444, emissive: 0x300808, shininess: 90 }),
  MAX_EACH
);
matterMesh.count = 0;
antiMesh.count   = 0;
scene.add(matterMesh, antiMesh);

// Simulation bounding box — visual reference for the 800³ toroidal cube.
scene.add(new THREE.LineSegments(
  new THREE.EdgesGeometry(new THREE.BoxGeometry(800, 800, 800)),
  new THREE.LineBasicMaterial({ color: 0x1a1a2a })
));

// ---------------------------------------------------------------------------
// Force line geometry — pre-allocated LineSegments with vertex colours
// ---------------------------------------------------------------------------
const FORCE_MAX_SEGS   = 6000;    // 2 verts per segment → float32 * 6
const FORCE_LOG_SCALE  = 600;
const FORCE_MAX_PX     = 45;      // max display length in sim units

const forceGeo   = new THREE.BufferGeometry();
const forcePosArr = new Float32Array(FORCE_MAX_SEGS * 2 * 3);
const forceColArr = new Float32Array(FORCE_MAX_SEGS * 2 * 3);
forceGeo.setAttribute('position', new THREE.BufferAttribute(forcePosArr, 3));
forceGeo.setAttribute('color',    new THREE.BufferAttribute(forceColArr, 3));
forceGeo.setDrawRange(0, 0);
const forceLines = new THREE.LineSegments(
  forceGeo,
  new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, opacity: 0.75 })
);
scene.add(forceLines);

// ---------------------------------------------------------------------------
// Fabric geometry — pre-allocated LineSegments with vertex colours
// ---------------------------------------------------------------------------
const FABRIC_MAX_SEGS = 8000;

const fabricGeo    = new THREE.BufferGeometry();
const fabricPosArr = new Float32Array(FABRIC_MAX_SEGS * 2 * 3);
const fabricColArr = new Float32Array(FABRIC_MAX_SEGS * 2 * 3);
fabricGeo.setAttribute('position', new THREE.BufferAttribute(fabricPosArr, 3));
fabricGeo.setAttribute('color',    new THREE.BufferAttribute(fabricColArr, 3));
fabricGeo.setDrawRange(0, 0);
const fabricLines = new THREE.LineSegments(
  fabricGeo,
  new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, opacity: 0.45 })
);
scene.add(fabricLines);

// ---------------------------------------------------------------------------
// Controls state
// ---------------------------------------------------------------------------
let paused        = false;
let particleCount = 500;
let sphereSize    = 3;
let showForces    = false;
let showFabric    = false;

const btnPlay     = document.getElementById('btn-play');
const btnReset    = document.getElementById('btn-reset');
const btnForces   = document.getElementById('btn-forces');
const btnFabric   = document.getElementById('btn-fabric');
const countSlider = document.getElementById('count');
const countVal    = document.getElementById('count-val');
const sphSlider   = document.getElementById('sphere-size');
const sphVal      = document.getElementById('sphere-val');
const forceWarn   = document.getElementById('force-warning');
const fabricWarn  = document.getElementById('fabric-warning');

function updateWarnings() {
  const warn = window.GRAV_ENV?.warn ?? { forceLines: 1000, fabric: 500 };
  if (showForces && particleCount > warn.forceLines) {
    forceWarn.textContent = `⚠ slow above ${warn.forceLines}`;
  } else {
    forceWarn.textContent = '';
  }
  if (showFabric && particleCount > warn.fabric) {
    fabricWarn.textContent = `⚠ slow above ${warn.fabric}`;
  } else {
    fabricWarn.textContent = '';
  }
}

// Reopen SSE with current query params whenever a toggle changes.
let evtSource = null;
function reconnectStream() {
  if (evtSource) evtSource.close();
  const params = new URLSearchParams();
  if (showForces) params.set('forces', '1');
  if (showFabric) params.set('fabric', '1');
  const url = '/sim/stream' + (params.toString() ? '?' + params : '');
  evtSource = new EventSource(url);
  evtSource.onmessage = onFrame;
  evtSource.onerror   = () => console.warn('SSE stream disconnected — browser will retry');
}

btnPlay.addEventListener('click', () => {
  paused = !paused;
  btnPlay.textContent = paused ? 'Play' : 'Pause';
  btnPlay.classList.toggle('active', !paused);
});

btnReset.addEventListener('click', () => {
  fetch(`/sim/reset?n=${particleCount}`, { method: 'POST' });
});

btnForces.addEventListener('click', () => {
  showForces = !showForces;
  btnForces.classList.toggle('active', showForces);
  if (!showForces) {
    forceGeo.setDrawRange(0, 0);
    forceGeo.attributes.position.needsUpdate = true;
  }
  updateWarnings();
  reconnectStream();
});

btnFabric.addEventListener('click', () => {
  showFabric = !showFabric;
  btnFabric.classList.toggle('active', showFabric);
  if (!showFabric) {
    fabricGeo.setDrawRange(0, 0);
    fabricGeo.attributes.position.needsUpdate = true;
  }
  updateWarnings();
  reconnectStream();
});

countSlider.addEventListener('input', () => {
  particleCount = parseInt(countSlider.value);
  countVal.textContent = particleCount;
  updateWarnings();
  fetch(`/sim/reset?n=${particleCount}`, { method: 'POST' });
});

sphSlider.addEventListener('input', () => {
  sphereSize = parseInt(sphSlider.value);
  sphVal.textContent = sphereSize;
});

// ---------------------------------------------------------------------------
// Particle update
// ---------------------------------------------------------------------------
const dummy = new THREE.Object3D();

function updateParticles(pArr) {
  let mi = 0, ai = 0;
  for (let i = 0; i < pArr.length; i += 5) {
    dummy.position.set(pArr[i], pArr[i + 1], pArr[i + 2]);
    dummy.scale.setScalar(Math.max(0.5, Math.sqrt(pArr[i + 4]) * sphereSize));
    dummy.updateMatrix();
    if (pArr[i + 3] > 0) {
      if (mi < MAX_EACH) matterMesh.setMatrixAt(mi++, dummy.matrix);
    } else {
      if (ai < MAX_EACH) antiMesh.setMatrixAt(ai++, dummy.matrix);
    }
  }
  matterMesh.count = mi;
  antiMesh.count   = ai;
  matterMesh.instanceMatrix.needsUpdate = true;
  antiMesh.instanceMatrix.needsUpdate   = true;
}

// ---------------------------------------------------------------------------
// Force lines update
// ---------------------------------------------------------------------------
// Matter = blue (0x4488ff), Anti = red (0xff4444)
const COL_MATTER = [0x44 / 255, 0x88 / 255, 1.0];
const COL_ANTI   = [1.0, 0x44 / 255, 0x44 / 255];

function updateForceLines(pArr, fArr) {
  if (!fArr || fArr.length === 0) { forceGeo.setDrawRange(0, 0); return; }
  const n = Math.min(pArr.length / 5, fArr.length / 3, FORCE_MAX_SEGS);
  let vi = 0;
  for (let i = 0; i < n; i++) {
    const px = pArr[i * 5], py = pArr[i * 5 + 1], pz = pArr[i * 5 + 2];
    const fx = fArr[i * 3], fy = fArr[i * 3 + 1], fz = fArr[i * 3 + 2];
    const fmag = Math.sqrt(fx * fx + fy * fy + fz * fz);
    if (fmag < 1e-12) continue;
    const len = Math.min(Math.log1p(fmag * FORCE_LOG_SCALE) * 10, FORCE_MAX_PX);
    const inv = len / fmag;
    const ex = px + fx * inv, ey = py + fy * inv, ez = pz + fz * inv;

    const col = pArr[i * 5 + 3] > 0 ? COL_MATTER : COL_ANTI;
    const base = vi * 6;
    forcePosArr[base]     = px; forcePosArr[base + 1] = py; forcePosArr[base + 2] = pz;
    forcePosArr[base + 3] = ex; forcePosArr[base + 4] = ey; forcePosArr[base + 5] = ez;
    forceColArr[base]     = col[0]; forceColArr[base + 1] = col[1]; forceColArr[base + 2] = col[2];
    forceColArr[base + 3] = col[0]; forceColArr[base + 4] = col[1]; forceColArr[base + 5] = col[2];
    vi++;
  }
  forceGeo.setDrawRange(0, vi * 2);
  forceGeo.attributes.position.needsUpdate = true;
  forceGeo.attributes.color.needsUpdate    = true;
}

// ---------------------------------------------------------------------------
// Fabric update
// ---------------------------------------------------------------------------
// Matter-matter=blue, anti-anti=red, mixed=amber
const COL_FAB_MM  = [0x33 / 255, 0x77 / 255, 0xff / 255];
const COL_FAB_AA  = [0xff / 255, 0x44 / 255, 0x44 / 255];
const COL_FAB_MIX = [1.0, 0xaa / 255, 0x44 / 255];

function updateFabric(fabArr) {
  if (!fabArr || fabArr.length === 0) { fabricGeo.setDrawRange(0, 0); return; }
  const n = Math.min(fabArr.length / 7, FABRIC_MAX_SEGS);
  for (let i = 0; i < n; i++) {
    const base = i * 6;
    fabricPosArr[base]     = fabArr[i * 7];
    fabricPosArr[base + 1] = fabArr[i * 7 + 1];
    fabricPosArr[base + 2] = fabArr[i * 7 + 2];
    fabricPosArr[base + 3] = fabArr[i * 7 + 3];
    fabricPosArr[base + 4] = fabArr[i * 7 + 4];
    fabricPosArr[base + 5] = fabArr[i * 7 + 5];
    const kind = fabArr[i * 7 + 6];
    const col  = kind === 1 ? COL_FAB_MM : kind === 2 ? COL_FAB_AA : COL_FAB_MIX;
    fabricColArr[base]     = col[0]; fabricColArr[base + 1] = col[1]; fabricColArr[base + 2] = col[2];
    fabricColArr[base + 3] = col[0]; fabricColArr[base + 4] = col[1]; fabricColArr[base + 5] = col[2];
  }
  fabricGeo.setDrawRange(0, n * 2);
  fabricGeo.attributes.position.needsUpdate = true;
  fabricGeo.attributes.color.needsUpdate    = true;
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------
const sMatter = document.getElementById('s-matter');
const sAnti   = document.getElementById('s-anti');
const sAnn    = document.getElementById('s-ann');
const sMerged = document.getElementById('s-merged');
const sSteps  = document.getElementById('s-steps');

function updateStats(s) {
  sMatter.textContent = s.matter;
  sAnti.textContent   = s.anti;
  sAnn.textContent    = s.ann;
  sMerged.textContent = s.merged;
  sSteps.textContent  = s.steps;
}

// ---------------------------------------------------------------------------
// SSE stream from Go physics server
// ---------------------------------------------------------------------------
function onFrame(e) {
  if (paused) return;
  const data = JSON.parse(e.data);
  updateParticles(data.p);
  if (showForces)  updateForceLines(data.p, data.f);
  if (showFabric)  updateFabric(data.fab);
  updateStats(data.s);
}

reconnectStream();

// ---------------------------------------------------------------------------
// Render loop
// ---------------------------------------------------------------------------
function animate() {
  requestAnimationFrame(animate);
  controls.update();
  renderer.render(scene, camera);
}
animate();
