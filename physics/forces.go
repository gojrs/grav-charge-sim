package physics

import "math"

const (
	// G is scaled for simulation units, not SI. Tune this to control how
	// "sticky" the gravity feels at the particle counts and box sizes we use.
	G         = 1.0
	Softening = 2.0 // prevents infinite force when two particles overlap
)

// gravitationalForce returns the force exerted on particle a by particle b,
// using the minimum image convention so the force is calculated across the
// shortest path on the toroidal box rather than the raw coordinate difference.
//
// The core of the conjecture lives here: multiplying by a.GCharge * b.GCharge
// makes same-type pairs attract (product = +1) and opposite-type pairs repel
// (product = -1). Remove that product and you get standard Newtonian gravity.
func gravitationalForce(a, b Particle, boxSize float64) Vector3 {
	// minImage gives the shortest displacement vector on the torus.
	diff := minImage(b.Position.Sub(a.Position), boxSize)
	distSq := diff.LengthSq()

	if distSq < Softening*Softening {
		distSq = Softening * Softening
	}

	dist := math.Sqrt(distSq)

	// q1*q2: the one sign that makes this conjecture different from Newton.
	forceMag := G * a.Mass * b.Mass * a.GCharge * b.GCharge / distSq

	return diff.Scale(forceMag / dist)
}

// minImage applies the minimum image convention to a displacement vector.
// For each axis: if the component is more than half a box-length away, the
// closer image across the periodic boundary is used instead.
// Precondition: positions have already been wrapped into [-L/2, L/2), so the
// raw difference is at most L in any axis — one if/else is sufficient.
func minImage(v Vector3, L float64) Vector3 {
	return Vector3{
		X: minImageScalar(v.X, L),
		Y: minImageScalar(v.Y, L),
		Z: minImageScalar(v.Z, L),
	}
}

func minImageScalar(d, L float64) float64 {
	if d > L/2 {
		return d - L
	}
	if d < -L/2 {
		return d + L
	}
	return d
}
