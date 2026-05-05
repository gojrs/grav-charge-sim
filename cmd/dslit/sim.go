package main

import "math"

const (
	gravG    = 1.0
	softenSq = 4.0
	simDT    = 0.016
)

type vec2 struct{ x, y float64 }

type body struct {
	pos    vec2
	vel    vec2
	mass   float64
	charge float64
}

// apparatus holds the gun, its antimatter anchors, and optional slit emitters.
// Emitter anchors are fixed; only gun↔emitter coupling is live when emitterOn.
type apparatus struct {
	gun       body
	topAnchor body
	botAnchor body

	emit1    body // matter body oscillating at top slit (y-constrained)
	emit1Top body // fixed antimatter anchor above emit1
	emit1Bot body // fixed antimatter anchor below emit1

	emit2    body // matter body oscillating at bottom slit (y-constrained)
	emit2Top body // fixed antimatter anchor above emit2
	emit2Bot body // fixed antimatter anchor below emit2

	emitterOn bool
}

func newApparatus(cfg ShotConfig) *apparatus {
	slit1 := cfg.SlitSepY / 2
	slit2 := -cfg.SlitSepY / 2
	ed := cfg.EmitterAnchorDist

	return &apparatus{
		gun:       body{pos: vec2{0, 0}, vel: vec2{0, cfg.GunInitVelY}, mass: 1.0, charge: +1},
		topAnchor: body{pos: vec2{0, +cfg.AnchorDistY}, mass: cfg.AnchorMass, charge: -1},
		botAnchor: body{pos: vec2{0, -cfg.AnchorDistY}, mass: cfg.AnchorMass, charge: -1},

		emit1:    body{pos: vec2{cfg.EmitterWallX, slit1}, mass: cfg.EmitterMass, charge: +1},
		emit1Top: body{pos: vec2{cfg.EmitterWallX, slit1 + ed}, mass: cfg.EmitterAnchorMass, charge: -1},
		emit1Bot: body{pos: vec2{cfg.EmitterWallX, slit1 - ed}, mass: cfg.EmitterAnchorMass, charge: -1},

		emit2:    body{pos: vec2{cfg.EmitterWallX, slit2}, mass: cfg.EmitterMass, charge: +1},
		emit2Top: body{pos: vec2{cfg.EmitterWallX, slit2 + ed}, mass: cfg.EmitterAnchorMass, charge: -1},
		emit2Bot: body{pos: vec2{cfg.EmitterWallX, slit2 - ed}, mass: cfg.EmitterAnchorMass, charge: -1},

		emitterOn: cfg.EmitterOn,
	}
}

// gravForce returns the grav-charge force on a from b.
// q1*q2 > 0 → attractive; q1*q2 < 0 → repulsive.
func gravForce(a, b body) vec2 {
	dx := b.pos.x - a.pos.x
	dy := b.pos.y - a.pos.y
	d2 := dx*dx + dy*dy
	if d2 < softenSq {
		d2 = softenSq
	}
	d := math.Sqrt(d2)
	f := gravG * a.mass * b.mass * a.charge * b.charge / d2
	return vec2{f * dx / d, f * dy / d}
}

func (app *apparatus) step() {
	// Gun: restoring force from its anchors + emitter coupling when on.
	fg1 := gravForce(app.gun, app.topAnchor)
	fg2 := gravForce(app.gun, app.botAnchor)
	gax := (fg1.x + fg2.x) / app.gun.mass
	gay := (fg1.y + fg2.y) / app.gun.mass

	if app.emitterOn {
		fe1 := gravForce(app.gun, app.emit1)
		fe2 := gravForce(app.gun, app.emit2)
		gax += (fe1.x + fe2.x) / app.gun.mass
		gay += (fe1.y + fe2.y) / app.gun.mass
	}

	app.gun.vel.x += gax * simDT
	app.gun.vel.y += gay * simDT
	app.gun.pos.x += app.gun.vel.x * simDT
	app.gun.pos.y += app.gun.vel.y * simDT

	if app.emitterOn {
		// Emitter 1: restored by its anchors, driven by gun coupling; x locked to wall.
		e1a1 := gravForce(app.emit1, app.emit1Top)
		e1a2 := gravForce(app.emit1, app.emit1Bot)
		e1g := gravForce(app.emit1, app.gun)
		e1ay := (e1a1.y + e1a2.y + e1g.y) / app.emit1.mass
		app.emit1.vel.y += e1ay * simDT
		app.emit1.pos.y += app.emit1.vel.y * simDT

		// Emitter 2: same, at bottom slit.
		e2a1 := gravForce(app.emit2, app.emit2Top)
		e2a2 := gravForce(app.emit2, app.emit2Bot)
		e2g := gravForce(app.emit2, app.gun)
		e2ay := (e2a1.y + e2a2.y + e2g.y) / app.emit2.mass
		app.emit2.vel.y += e2ay * simDT
		app.emit2.pos.y += app.emit2.vel.y * simDT
	}
}

// ShotConfig defines one simulation run.
type ShotConfig struct {
	StepsPerShot      int     `json:"stepsPerShot"`
	NumShots          int     `json:"numShots"`
	SlitSepY          float64 `json:"slitSepY"`
	SlitWidth         float64 `json:"slitWidth"`
	GunInitVelY       float64 `json:"gunInitVelY"`
	AnchorMass        float64 `json:"anchorMass"`
	AnchorDistY       float64 `json:"anchorDistY"`
	EmitterOn         bool    `json:"emitterOn"`
	EmitterWallX      float64 `json:"emitterWallX"`      // x-position of slit wall
	EmitterMass       float64 `json:"emitterMass"`       // emitter body mass
	EmitterAnchorDist float64 `json:"emitterAnchorDist"` // anchor y-distance from slit center
	EmitterAnchorMass float64 `json:"emitterAnchorMass"` // ω_emit = sqrt(4·G·M/A³)
}

func DefaultShotConfig() ShotConfig {
	return ShotConfig{
		StepsPerShot:      10,
		NumShots:          10000,
		SlitSepY:          20.0,
		SlitWidth:         5.0,
		GunInitVelY:       5.0,
		AnchorMass:        500.0,
		AnchorDistY:       30.0,
		EmitterOn:         false,
		EmitterWallX:      50.0,
		EmitterMass:       1.0,
		EmitterAnchorDist: 15.0,
		EmitterAnchorMass: 500.0,
	}
}

const (
	histMin  = -30.0
	histMax  = 30.0
	histBins = 60
	binWidth = (histMax - histMin) / histBins
)

// RunResult holds the screen distribution and metadata for one run.
type RunResult struct {
	Config     ShotConfig `json:"config"`
	HitCount   int        `json:"hitCount"`
	BlockCount int        `json:"blockCount"`
	HitRate    float64    `json:"hitRate"`
	BinEdges   []float64  `json:"binEdges"`
	BinCounts  []int      `json:"binCounts"`
	BinDensity []float64  `json:"binDensity"`
	OscPeriod  float64    `json:"oscPeriod"` // gun oscillation period in steps
	OscAmp     float64    `json:"oscAmp"`    // observed peak gun y-displacement
	EmitAmp    float64    `json:"emitAmp"`   // peak emitter1 y-displacement from slit center
}

// RunShots executes the simulation and returns the screen hit distribution.
func RunShots(cfg ShotConfig) RunResult {
	app := newApparatus(cfg)

	k := 4 * gravG * cfg.AnchorMass / (cfg.AnchorDistY * cfg.AnchorDistY * cfg.AnchorDistY)
	omega := math.Sqrt(k)
	oscPeriod := 2 * math.Pi / omega / simDT

	counts := make([]int, histBins)
	hitCount, blockCount := 0, 0

	slit1 := cfg.SlitSepY / 2
	slit2 := -cfg.SlitSepY / 2
	halfW := cfg.SlitWidth / 2

	maxY := 0.0
	maxEmitDisp := 0.0
	emit1Rest := cfg.SlitSepY / 2

	for shot := 0; shot < cfg.NumShots; shot++ {
		for s := 0; s < cfg.StepsPerShot; s++ {
			app.step()
		}

		y := app.gun.pos.y
		if math.Abs(y) > maxY {
			maxY = math.Abs(y)
		}

		if cfg.EmitterOn {
			d := math.Abs(app.emit1.pos.y - emit1Rest)
			if d > maxEmitDisp {
				maxEmitDisp = d
			}
		}

		if math.Abs(y-slit1) <= halfW || math.Abs(y-slit2) <= halfW {
			hitCount++
			bin := int((y - histMin) / binWidth)
			if bin >= 0 && bin < histBins {
				counts[bin]++
			}
		} else {
			blockCount++
		}
	}

	edges := make([]float64, histBins)
	for i := range edges {
		edges[i] = histMin + float64(i)*binWidth
	}

	density := make([]float64, histBins)
	if hitCount > 0 {
		for i, c := range counts {
			density[i] = float64(c) / (float64(hitCount) * binWidth)
		}
	}

	return RunResult{
		Config:     cfg,
		HitCount:   hitCount,
		BlockCount: blockCount,
		HitRate:    float64(hitCount) / float64(cfg.NumShots),
		BinEdges:   edges,
		BinCounts:  counts,
		BinDensity: density,
		OscPeriod:  oscPeriod,
		OscAmp:     maxY,
		EmitAmp:    maxEmitDisp,
	}
}
