package physics

import "math"

// ParticleIndex is an index into Simulation.Particles.
type ParticleIndex = int

// spatialHash maps 2D cell keys to the particle indices in that cell.
// Only XY coordinates are used; Z is ignored (collision detection is 2D).
//
// Structure: map[int][]ParticleIndex
// Hash:      floor((x+half)/bucketSize) + floor((y+half)/bucketSize) * gridWidth
//
// A 3×3 neighborhood query guarantees all particles within bucketSize on both
// axes are found, so callers must set bucketSize ≥ their interaction radius.
type spatialHash struct {
	cells      map[int][]ParticleIndex
	bucketSize float64
	gridWidth  int
	half       float64 // boxSize / 2, cached
}

func newSpatialHash(bucketSize, boxSize float64) spatialHash {
	// +1 ensures positions at exactly ±half land in a valid cell.
	gridWidth := int(math.Ceil(boxSize/bucketSize)) + 1
	return spatialHash{
		cells:      make(map[int][]ParticleIndex),
		bucketSize: bucketSize,
		gridWidth:  gridWidth,
		half:       boxSize / 2,
	}
}

func (h *spatialHash) cellCoords(p Particle) (cx, cy int) {
	// Shift from [-L/2, L/2) → [0, L) before flooring.
	cx = int(math.Floor((p.Position.X + h.half) / h.bucketSize))
	cy = int(math.Floor((p.Position.Y + h.half) / h.bucketSize))
	return
}

func (h *spatialHash) cellKey(cx, cy int) int {
	gw := h.gridWidth
	// Wrap negative coords back into [0, gw) for toroidal boundary.
	cx = ((cx % gw) + gw) % gw
	cy = ((cy % gw) + gw) % gw
	return cx + cy*gw
}

func (h *spatialHash) build(particles []Particle) {
	for i := range particles {
		cx, cy := h.cellCoords(particles[i])
		k := h.cellKey(cx, cy)
		h.cells[k] = append(h.cells[k], i)
	}
}

// candidates returns all particle indices in the 3×3 cell neighborhood of p.
// All particles within bucketSize of p on both axes are guaranteed to appear.
func (h *spatialHash) candidates(p Particle) []ParticleIndex {
	cx, cy := h.cellCoords(p)
	var out []ParticleIndex
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			out = append(out, h.cells[h.cellKey(cx+dx, cy+dy)]...)
		}
	}
	return out
}
