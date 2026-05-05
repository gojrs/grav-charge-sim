package physics

import (
	"math"
	"math/rand"
	"sync"
)

const (
	AnnihilationRadius  = 3.0  // opposite-charge pairs closer than this annihilate
	MergeRadius         = 5.0  // same-charge pairs closer than this merge
	PocketRatioThreshold = 10.0 // mass ratio that qualifies a merge as a "pocket"
)

// PocketEvent records a merge where one body's mass dominated the other by more than PocketRatioThreshold.
type PocketEvent struct {
	X, Y  float64
	Ratio float64 // larger/smaller mass ratio at the moment of merge
}

// Config holds the initial conditions for a simulation run.
type Config struct {
	N                int     // total particle count (split evenly matter/antimatter)
	BoxSize          float64 // particles start within [-BoxSize/2, BoxSize/2] on each axis
	MaxSpeed         float64 // magnitude cap on random initial velocity
	ParticleMass     float64
	ThreeDimensional bool // initialise Z coords and velocities; use Step3D for correct 3D forces
}

// DefaultConfig returns sensible starting values for a 500-particle run.
func DefaultConfig() Config {
	return Config{
		N:            500,
		BoxSize:      800,
		MaxSpeed:     10,
		ParticleMass: 1.0,
	}
}

// Simulation holds all live particles and running counters.
type Simulation struct {
	Particles         []Particle
	AnnihilationCount int
	MergeCount        int
	StepCount         int
	PocketEvents      []PocketEvent
	cfg               Config
}

// New creates a simulation with cfg.N particles placed randomly inside the box,
// split 50/50 between matter and antimatter.
func New(cfg Config) *Simulation {
	particles := make([]Particle, cfg.N)
	half := cfg.BoxSize / 2

	for i := range particles {
		charge := Matter
		if i%2 != 0 {
			charge = Antimatter
		}
		zPos, zVel := 0.0, 0.0
		if cfg.ThreeDimensional {
			zPos = (rand.Float64()*2 - 1) * half
			zVel = (rand.Float64()*2 - 1) * cfg.MaxSpeed
		}
		particles[i] = Particle{
			Position: Vector3{
				X: (rand.Float64()*2 - 1) * half,
				Y: (rand.Float64()*2 - 1) * half,
				Z: zPos,
			},
			Velocity: Vector3{
				X: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Y: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Z: zVel,
			},
			Mass:    cfg.ParticleMass,
			GCharge: charge,
		}
	}

	return &Simulation{Particles: particles, cfg: cfg}
}

// Step advances the simulation by dt seconds using a simple Euler integrator.
// Forces are computed O(N log N) via Barnes-Hut quadtree, then positions are
// integrated and opposite-charge pairs within AnnihilationRadius are removed.
func (s *Simulation) Step(dt float64) {
	n := len(s.Particles)
	forces := make([]Vector3, n)

	// Build quadtree and compute forces — O(N log N).
	if n > 0 {
		root := buildTree(s.Particles, s.cfg.BoxSize)
		for i := range s.Particles {
			forces[i] = root.force(&s.Particles[i], s.cfg.BoxSize)
		}
	}

	// Integrate, then wrap positions back into [-BoxSize/2, BoxSize/2).
	for i := range s.Particles {
		s.Particles[i].ApplyForce(forces[i], dt)
		s.Particles[i].Integrate(dt)
		s.Particles[i].Position = wrapPosition(s.Particles[i].Position, s.cfg.BoxSize)
	}

	s.annihilate()
	s.merge()
	s.StepCount++
}

// ForcePair is a pairwise interaction above a force threshold, in simulation coords.
// Kind: 1 = matter-matter, 2 = anti-anti, 3 = matter-anti.
// AZ/BZ are zero for 2D simulations.
type ForcePair struct {
	AX, AY, AZ float64
	BX, BY, BZ float64
	Kind       float64
}

// ComputeSplitForces separates per-particle forces into attractive (same-charge)
// and repulsive (opposite-charge) components via O(N²) pairwise evaluation.
// Intended for small-N force-line visualization — not used in the simulation step.
func (s *Simulation) ComputeSplitForces() (attractive, repulsive []Vector3) {
	n := len(s.Particles)
	attractive = make([]Vector3, n)
	repulsive = make([]Vector3, n)
	for i := range s.Particles {
		for j := range s.Particles {
			if i == j {
				continue
			}
			f := gravitationalForce(s.Particles[i], s.Particles[j], s.cfg.BoxSize)
			if s.Particles[i].GCharge == s.Particles[j].GCharge {
				attractive[i] = attractive[i].Add(f)
			} else {
				repulsive[i] = repulsive[i].Add(f)
			}
		}
	}
	return
}

// ComputeFabricPairs returns all same-step pairwise interactions whose force
// magnitude exceeds threshold. O(N²) — intended for small-N fabric visualization.
func (s *Simulation) ComputeFabricPairs(threshold float64) []ForcePair {
	n := len(s.Particles)
	var pairs []ForcePair
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			f := gravitationalForce(s.Particles[i], s.Particles[j], s.cfg.BoxSize)
			if f.Length() < threshold {
				continue
			}
			var kind float64
			switch {
			case s.Particles[i].GCharge > 0 && s.Particles[j].GCharge > 0:
				kind = 1
			case s.Particles[i].GCharge < 0 && s.Particles[j].GCharge < 0:
				kind = 2
			default:
				kind = 3
			}
			pairs = append(pairs, ForcePair{
				AX: s.Particles[i].Position.X, AY: s.Particles[i].Position.Y, AZ: s.Particles[i].Position.Z,
				BX: s.Particles[j].Position.X, BY: s.Particles[j].Position.Y, BZ: s.Particles[j].Position.Z,
				Kind: kind,
			})
		}
	}
	return pairs
}

// ComputeForces3D returns the net gravitational force on each particle via O(N²)
// pairwise evaluation. Intended for small-N force-line visualization in 3D.
func (s *Simulation) ComputeForces3D() []Vector3 {
	n := len(s.Particles)
	forces := make([]Vector3, n)
	for i := range s.Particles {
		for j := range s.Particles {
			if i == j {
				continue
			}
			forces[i] = forces[i].Add(gravitationalForce(s.Particles[i], s.Particles[j], s.cfg.BoxSize))
		}
	}
	return forces
}

// Step3D advances the simulation using O(N²) pairwise forces in full 3D.
// The Barnes-Hut Step() is 2D-only (the quadtree ignores Z); use this method
// when ThreeDimensional is true. Workers goroutines parallelize the force phase.
func (s *Simulation) Step3D(dt float64, workers int) {
	n := len(s.Particles)
	forces := make([]Vector3, n)

	if n > 0 {
		var wg sync.WaitGroup
		chunk := (n + workers - 1) / workers
		for w := 0; w < workers; w++ {
			lo := w * chunk
			hi := min(lo+chunk, n)
			if lo >= n {
				break
			}
			wg.Add(1)
			go func(lo, hi int) {
				defer wg.Done()
				for i := lo; i < hi; i++ {
					for j := range s.Particles {
						if i == j {
							continue
						}
						forces[i] = forces[i].Add(gravitationalForce(s.Particles[i], s.Particles[j], s.cfg.BoxSize))
					}
				}
			}(lo, hi)
		}
		wg.Wait()
	}

	for i := range s.Particles {
		s.Particles[i].ApplyForce(forces[i], dt)
		s.Particles[i].Integrate(dt)
		s.Particles[i].Position = wrapPosition(s.Particles[i].Position, s.cfg.BoxSize)
	}

	s.annihilate()
	s.merge()
	s.StepCount++
}

// ComputeForces returns the net gravitational force on each particle without
// advancing the simulation. Used by the WASM layer for force-line visualization.
func (s *Simulation) ComputeForces() []Vector3 {
	n := len(s.Particles)
	forces := make([]Vector3, n)
	if n > 0 {
		root := buildTree(s.Particles, s.cfg.BoxSize)
		for i := range s.Particles {
			forces[i] = root.force(&s.Particles[i], s.cfg.BoxSize)
		}
	}
	return forces
}

// StepConcurrent advances the simulation by dt, distributing force calculations
// across workers goroutines. The Barnes-Hut tree is built once; each goroutine
// handles a contiguous slice of particles. Safe to call from a single goroutine.
func (s *Simulation) StepConcurrent(dt float64, workers int) {
	n := len(s.Particles)
	forces := make([]Vector3, n)

	if n > 0 {
		root := buildTree(s.Particles, s.cfg.BoxSize)

		var wg sync.WaitGroup
		chunk := (n + workers - 1) / workers
		for w := 0; w < workers; w++ {
			lo := w * chunk
			hi := lo + chunk
			if hi > n {
				hi = n
			}
			if lo >= n {
				break
			}
			wg.Add(1)
			go func(lo, hi int) {
				defer wg.Done()
				for i := lo; i < hi; i++ {
					forces[i] = root.force(&s.Particles[i], s.cfg.BoxSize)
				}
			}(lo, hi)
		}
		wg.Wait()
	}

	for i := range s.Particles {
		s.Particles[i].ApplyForce(forces[i], dt)
		s.Particles[i].Integrate(dt)
		s.Particles[i].Position = wrapPosition(s.Particles[i].Position, s.cfg.BoxSize)
	}

	s.annihilate()
	s.merge()
	s.StepCount++
}

// annihilate removes matter/antimatter pairs within AnnihilationRadius.
// Spatial hash reduces the search from O(N²) to O(N·k) where k is the
// average bucket occupancy.
func (s *Simulation) annihilate() {
	n := len(s.Particles)
	dead := make([]bool, n)

	h := newSpatialHash(AnnihilationRadius, s.cfg.BoxSize)
	h.build(s.Particles)

	for i := 0; i < n; i++ {
		if dead[i] {
			continue
		}
		for _, j := range h.candidates(s.Particles[i]) {
			if j <= i || dead[j] {
				continue
			}
			if s.Particles[i].GCharge == s.Particles[j].GCharge {
				continue
			}
			diff := minImage(s.Particles[i].Position.Sub(s.Particles[j].Position), s.cfg.BoxSize)
			if diff.Length() < AnnihilationRadius {
				dead[i] = true
				dead[j] = true
				s.AnnihilationCount++
				break
			}
		}
	}

	live := s.Particles[:0]
	for i, p := range s.Particles {
		if !dead[i] {
			live = append(live, p)
		}
	}
	s.Particles = live
}

// merge combines same-charge pairs within MergeRadius.
// Spatial hash reduces the search from O(N²) to O(N·k) where k is the
// average bucket occupancy.
func (s *Simulation) merge() {
	n := len(s.Particles)
	dead := make([]bool, n)

	h := newSpatialHash(MergeRadius, s.cfg.BoxSize)
	h.build(s.Particles)

	for i := 0; i < n; i++ {
		if dead[i] {
			continue
		}
		for _, j := range h.candidates(s.Particles[i]) {
			if j <= i || dead[j] {
				continue
			}
			if s.Particles[i].GCharge != s.Particles[j].GCharge {
				continue
			}
			diff := minImage(s.Particles[i].Position.Sub(s.Particles[j].Position), s.cfg.BoxSize)
			if diff.Length() >= MergeRadius {
				continue
			}

			pi := &s.Particles[i]
			pj := &s.Particles[j]
			totalMass := pi.Mass + pj.Mass

			ratio := pi.Mass / pj.Mass
			if ratio < 1 {
				ratio = 1 / ratio
			}

			pi.Velocity = pi.Velocity.Scale(pi.Mass / totalMass).Add(pj.Velocity.Scale(pj.Mass / totalMass))

			toJ := minImage(pj.Position.Sub(pi.Position), s.cfg.BoxSize)
			pi.Position = wrapPosition(pi.Position.Add(toJ.Scale(pj.Mass/totalMass)), s.cfg.BoxSize)

			pi.Mass = totalMass
			dead[j] = true
			s.MergeCount++

			if ratio >= PocketRatioThreshold {
				s.PocketEvents = append(s.PocketEvents, PocketEvent{
					X: pi.Position.X, Y: pi.Position.Y, Ratio: ratio,
				})
			}
			break
		}
	}

	live := s.Particles[:0]
	for i, p := range s.Particles {
		if !dead[i] {
			live = append(live, p)
		}
	}
	s.Particles = live
}

// DrainPocketEvents returns and clears the accumulated pocket events so the JS
// layer can collect them each frame without the buffer growing unbounded.
func (s *Simulation) DrainPocketEvents() []PocketEvent {
	events := s.PocketEvents
	s.PocketEvents = nil
	return events
}

// wrapPosition maps a position back into the canonical box [-L/2, L/2) on
// every axis using the floor formula, which handles any displacement magnitude.
func wrapPosition(v Vector3, L float64) Vector3 {
	return Vector3{
		X: wrapScalar(v.X, L),
		Y: wrapScalar(v.Y, L),
		Z: wrapScalar(v.Z, L),
	}
}

func wrapScalar(x, L float64) float64 {
	// Shift so the box starts at 0, apply modulo, shift back.
	return x - L*math.Floor((x+L/2)/L)
}
