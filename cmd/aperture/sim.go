package main

import "math"

const (
	gravG    = 1.0
	softenSq = 4.0
	simDT    = 0.016
	histBins = 50
)

type vec2 struct{ x, y float64 }

type body struct {
	pos    vec2
	vel    vec2
	mass   float64
	charge float64
}

type emitterPoint struct {
	body
	topAnchor body
	botAnchor body
	restY     float64
}

type apparatus struct {
	gun       body
	topAnchor body
	botAnchor body
	emitters  []emitterPoint
	cfg       ShotConfig
}

func newApparatus(cfg ShotConfig) *apparatus {
	eps := make([]emitterPoint, len(cfg.EmitterYs))
	ed := cfg.EmitterAnchorDist
	for i, ey := range cfg.EmitterYs {
		eps[i] = emitterPoint{
			body:      body{pos: vec2{cfg.WallX, ey}, mass: 1.0, charge: +1},
			topAnchor: body{pos: vec2{cfg.WallX, ey + ed}, mass: cfg.EmitterAnchorMass, charge: -1},
			botAnchor: body{pos: vec2{cfg.WallX, ey - ed}, mass: cfg.EmitterAnchorMass, charge: -1},
			restY:     ey,
		}
	}
	return &apparatus{
		gun:       body{pos: vec2{0, 0}, vel: vec2{0, cfg.GunInitVelY}, mass: 1.0, charge: +1},
		topAnchor: body{pos: vec2{0, +cfg.AnchorDistY}, mass: cfg.AnchorMass, charge: -1},
		botAnchor: body{pos: vec2{0, -cfg.AnchorDistY}, mass: cfg.AnchorMass, charge: -1},
		emitters:  eps,
		cfg:       cfg,
	}
}

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

// waveForceY returns the y-component of the surface wave field force on the gun.
// F_y = strength × (apertureWidth / r)^falloffRate × (dy / r)
// y-only model: surface wave creates transverse displacement, no x-force applied to gun.
func waveForceY(gunPos, emitPos vec2, strength, apertureWidth, falloffRate float64) float64 {
	dx := emitPos.x - gunPos.x
	dy := emitPos.y - gunPos.y
	d2 := dx*dx + dy*dy
	if d2 < softenSq {
		d2 = softenSq
	}
	d := math.Sqrt(d2)
	return strength * math.Pow(apertureWidth/d, falloffRate) * (dy / d)
}

func (app *apparatus) step() {
	fg1 := gravForce(app.gun, app.topAnchor)
	fg2 := gravForce(app.gun, app.botAnchor)
	gax := (fg1.x + fg2.x) / app.gun.mass
	gay := (fg1.y + fg2.y) / app.gun.mass

	if app.cfg.EmitterOn {
		aw := app.cfg.ApertureHalfW * 2
		for i := range app.emitters {
			gay += waveForceY(app.gun.pos, app.emitters[i].pos,
				app.cfg.EmissionStrength, aw, app.cfg.FalloffRate) / app.gun.mass
		}
	}

	app.gun.vel.x += gax * simDT
	app.gun.vel.y += gay * simDT
	app.gun.pos.x += app.gun.vel.x * simDT
	app.gun.pos.y += app.gun.vel.y * simDT

	if app.cfg.EmitterOn {
		for i := range app.emitters {
			e := &app.emitters[i]
			ea1 := gravForce(e.body, e.topAnchor)
			ea2 := gravForce(e.body, e.botAnchor)
			eg := gravForce(e.body, app.gun)
			ay := (ea1.y + ea2.y + eg.y) / e.mass
			e.vel.y += ay * simDT
			e.pos.y += e.vel.y * simDT
		}
	}
}

// ShotConfig defines one simulation run.
type ShotConfig struct {
	StepsPerShot      int       `json:"stepsPerShot"`
	NumShots          int       `json:"numShots"`
	ApertureHalfW     float64   `json:"apertureHalfW"`
	WallX             float64   `json:"wallX"`
	GunInitVelY       float64   `json:"gunInitVelY"`
	AnchorMass        float64   `json:"anchorMass"`
	AnchorDistY       float64   `json:"anchorDistY"`
	EmitterOn         bool      `json:"emitterOn"`
	EmitterYs         []float64 `json:"emitterYs"`
	EmissionStrength  float64   `json:"emissionStrength"`
	FalloffRate       float64   `json:"falloffRate"`
	EmitterAnchorDist float64   `json:"emitterAnchorDist"`
	EmitterAnchorMass float64   `json:"emitterAnchorMass"`
}

func DefaultShotConfig() ShotConfig {
	return ShotConfig{
		StepsPerShot:      10,
		NumShots:          10000,
		ApertureHalfW:     2.5,
		WallX:             50.0,
		GunInitVelY:       5.0,
		AnchorMass:        500.0,
		AnchorDistY:       30.0,
		EmitterOn:         false,
		EmitterYs:         []float64{-2.5, 2.5},
		EmissionStrength:  10.0,
		FalloffRate:       1.0,
		EmitterAnchorDist: 15.0,
		EmitterAnchorMass: 500.0,
	}
}

// kEmitRatio returns k_emitter / k_gun — ratio of wave restoring stiffness to gun anchor
// stiffness at y=0. Computed numerically. Documented per emitter_constraint.
func kEmitRatio(cfg ShotConfig) float64 {
	if len(cfg.EmitterYs) == 0 {
		return 0
	}
	aw := cfg.ApertureHalfW * 2
	eps := 1e-5
	netFy := func(y float64) float64 {
		total := 0.0
		for _, ey := range cfg.EmitterYs {
			dy := ey - y
			d2 := cfg.WallX*cfg.WallX + dy*dy
			if d2 < softenSq {
				d2 = softenSq
			}
			d := math.Sqrt(d2)
			total += cfg.EmissionStrength * math.Pow(aw/d, cfg.FalloffRate) * (dy / d)
		}
		return total
	}
	kEmit := -(netFy(eps) - netFy(-eps)) / (2 * eps)
	kGun := 4 * gravG * cfg.AnchorMass / (cfg.AnchorDistY * cfg.AnchorDistY * cfg.AnchorDistY)
	if kGun <= 0 {
		return 0
	}
	return kEmit / kGun
}

// strengthMin returns the emission strength for 1 bin-width deflection at aperture edge
// distance per stepsPerShot interval. Pre-run documentation per emitter_constraint.
func strengthMin(cfg ShotConfig) float64 {
	h := cfg.ApertureHalfW
	aw := h * 2
	dt := float64(cfg.StepsPerShot) * simDT
	bw := aw / float64(histBins)
	rEdge := math.Sqrt(cfg.WallX*cfg.WallX + h*h)
	uy := h / rEdge
	denom := math.Pow(aw/rEdge, cfg.FalloffRate) * uy * dt * dt / 2
	if denom <= 0 {
		return 0
	}
	return bw / denom
}

// RunResult holds the aperture screen distribution and metadata for one run.
type RunResult struct {
	Config      ShotConfig `json:"config"`
	HitCount    int        `json:"hitCount"`
	BlockCount  int        `json:"blockCount"`
	HitRate     float64    `json:"hitRate"`
	BinEdges    []float64  `json:"binEdges"`
	BinCounts   []int      `json:"binCounts"`
	BinDensity  []float64  `json:"binDensity"`
	OscPeriod   float64    `json:"oscPeriod"`  // gun oscillation period in steps
	OscAmp      float64    `json:"oscAmp"`     // observed peak gun y-displacement
	EmitAmps    []float64  `json:"emitAmps"`   // peak emitter displacement per point
	StrengthMin float64    `json:"strengthMin"` // 1-bin deflection threshold (documented)
	KRatio      float64    `json:"kRatio"`     // k_emit / k_gun at y=0
	AEff        float64    `json:"aEff"`       // predicted effective gun amplitude
}

// RunShots executes the simulation and returns the aperture hit distribution.
func RunShots(cfg ShotConfig) RunResult {
	app := newApparatus(cfg)

	kGun := 4 * gravG * cfg.AnchorMass / (cfg.AnchorDistY * cfg.AnchorDistY * cfg.AnchorDistY)
	omega := math.Sqrt(kGun)
	oscPeriod := 2 * math.Pi / omega / simDT

	kr := 0.0
	aeff := cfg.GunInitVelY / omega
	if cfg.EmitterOn {
		kr = kEmitRatio(cfg)
		kTot := kGun * (1 + kr)
		if kTot > 0 {
			aeff = cfg.GunInitVelY / math.Sqrt(kTot)
		}
	}

	h := cfg.ApertureHalfW
	aw := h * 2
	bw := aw / float64(histBins)
	histMin := -h

	counts := make([]int, histBins)
	hitCount, blockCount := 0, 0
	maxY := 0.0
	maxEmitDisp := make([]float64, len(cfg.EmitterYs))

	for shot := 0; shot < cfg.NumShots; shot++ {
		for s := 0; s < cfg.StepsPerShot; s++ {
			app.step()
		}

		y := app.gun.pos.y
		if math.Abs(y) > maxY {
			maxY = math.Abs(y)
		}

		if cfg.EmitterOn {
			for i := range app.emitters {
				d := math.Abs(app.emitters[i].pos.y - app.emitters[i].restY)
				if d > maxEmitDisp[i] {
					maxEmitDisp[i] = d
				}
			}
		}

		if math.Abs(y) <= h {
			hitCount++
			bin := int((y - histMin) / bw)
			if bin >= 0 && bin < histBins {
				counts[bin]++
			}
		} else {
			blockCount++
		}
	}

	edges := make([]float64, histBins)
	for i := range edges {
		edges[i] = histMin + float64(i)*bw
	}

	density := make([]float64, histBins)
	if hitCount > 0 {
		for i, c := range counts {
			density[i] = float64(c) / (float64(hitCount) * bw)
		}
	}

	return RunResult{
		Config:      cfg,
		HitCount:    hitCount,
		BlockCount:  blockCount,
		HitRate:     float64(hitCount) / float64(cfg.NumShots),
		BinEdges:    edges,
		BinCounts:   counts,
		BinDensity:  density,
		OscPeriod:   oscPeriod,
		OscAmp:      maxY,
		EmitAmps:    maxEmitDisp,
		StrengthMin: strengthMin(cfg),
		KRatio:      kr,
		AEff:        aeff,
	}
}
