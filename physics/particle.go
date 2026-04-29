package physics

// GCharge values for the two particle types.
const (
	Matter     float64 = +1.0
	Antimatter float64 = -1.0
)

// Particle is a single body in the simulation.
//
// GCharge encodes the gravitational charge conjecture:
//   +1  (matter)     attracts other matter, repels antimatter
//   -1  (antimatter) attracts other antimatter, repels matter
//
// The sign of GCharge flips the direction of the gravitational force between
// opposite-charge pairs — that single sign change is the entire conjecture.
type Particle struct {
	Position Vector3
	Velocity Vector3
	Mass     float64
	GCharge  float64 // +1.0 matter, -1.0 antimatter
}

// ApplyForce updates Velocity using F = ma → a = F/m over timestep dt.
func (p *Particle) ApplyForce(force Vector3, dt float64) {
	acceleration := force.Scale(1 / p.Mass)
	p.Velocity = p.Velocity.Add(acceleration.Scale(dt))
}

// Integrate advances Position by Velocity over timestep dt.
func (p *Particle) Integrate(dt float64) {
	p.Position = p.Position.Add(p.Velocity.Scale(dt))
}

// IsMatter reports whether the particle is a matter particle.
func (p *Particle) IsMatter() bool {
	return p.GCharge > 0
}
