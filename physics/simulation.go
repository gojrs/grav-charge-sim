package physics

import (
	"math"
	"math/rand"
)

const AnnihilationRadius = 3.0 // particles closer than this annihilate

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
	Particles        []Particle
	AnnihilationCount int
	StepCount        int
	cfg              Config
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
				Z: (rand.Float64()*2 - 1) * half,
			},
			Velocity: Vector3{
				X: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Y: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
				Z: (rand.Float64()*2 - 1) * cfg.MaxSpeed,
			},
			Mass:    cfg.ParticleMass,
			GCharge: charge,
		}
	}

	return &Simulation{Particles: particles, cfg: cfg}
}

// Step advances the simulation by dt seconds using a simple Euler integrator.
// It accumulates forces O(N²), integrates velocities and positions, then
// removes any matter/antimatter pairs that have come within AnnihilationRadius.
func (s *Simulation) Step(dt float64) {
	n := len(s.Particles)
	forces := make([]Vector3, n)

	// Accumulate pairwise forces.
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			f := gravitationalForce(s.Particles[i], s.Particles[j], s.cfg.BoxSize)
			forces[i] = forces[i].Add(f)
			forces[j] = forces[j].Add(f.Scale(-1)) // Newton's third law
		}
	}

	// Integrate, then wrap positions back into [-BoxSize/2, BoxSize/2).
	for i := range s.Particles {
		s.Particles[i].ApplyForce(forces[i], dt)
		s.Particles[i].Integrate(dt)
		s.Particles[i].Position = wrapPosition(s.Particles[i].Position, s.cfg.BoxSize)
	}

	s.annihilate()
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
