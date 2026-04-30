package physics

import (
	"math"
	"math/rand"
)

const (
	AnnihilationRadius = 3.0 // opposite-charge pairs closer than this annihilate
	MergeRadius        = 5.0 // same-charge pairs closer than this merge
)

// Config holds the initial conditions for a simulation run.
type Config struct {
	N         int     // total particle count (split evenly matter/antimatter)
	BoxSize   float64 // particles start within [-BoxSize/2, BoxSize/2] on each axis
	MaxSpeed  float64 // magnitude cap on random initial velocity
	ParticleMass float64
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
		particles[i] = Particle{
			Position: Vector3{
				X: (rand.Float64()*2 - 1) * half,
				Y: (rand.Float64()*2 - 1) * half,
				Z: 0, // 2D simulation; octree + 3D mode is a future extension
			},
			Velocity: Vector3{
				X: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Y: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Z: 0,
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

// annihilate removes matter/antimatter pairs that are within AnnihilationRadius
// of each other. Each pair is counted once and both particles are removed.
func (s *Simulation) annihilate() {
	n := len(s.Particles)
	dead := make([]bool, n)

	for i := 0; i < n; i++ {
		if dead[i] {
			continue
		}
		for j := i + 1; j < n; j++ {
			if dead[j] {
				continue
			}
			// Only opposite-charge pairs annihilate.
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

	// Compact the slice, removing dead particles.
	live := s.Particles[:0]
	for i, p := range s.Particles {
		if !dead[i] {
			live = append(live, p)
		}
	}
	s.Particles = live
}

// merge combines same-charge pairs that are within MergeRadius of each other.
// The result has combined mass, momentum-conserving velocity, and a
// mass-weighted position computed via minimum image (handles toroidal wrap).
// This simulates gravitational collapse within matter and antimatter domains.
func (s *Simulation) merge() {
	n := len(s.Particles)
	dead := make([]bool, n)

	for i := 0; i < n; i++ {
		if dead[i] {
			continue
		}
		for j := i + 1; j < n; j++ {
			if dead[j] {
				continue
			}
			// Only same-charge particles merge.
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

			// Momentum-conserving velocity (uses original masses).
			pi.Velocity = pi.Velocity.Scale(pi.Mass / totalMass).Add(pj.Velocity.Scale(pj.Mass / totalMass))

			// Mass-weighted position via minimum image so wrap-around is handled.
			toJ := minImage(pj.Position.Sub(pi.Position), s.cfg.BoxSize)
			pi.Position = wrapPosition(pi.Position.Add(toJ.Scale(pj.Mass/totalMass)), s.cfg.BoxSize)

			pi.Mass = totalMass
			dead[j] = true
			s.MergeCount++
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
