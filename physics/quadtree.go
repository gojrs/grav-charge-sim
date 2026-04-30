package physics

import "math"

// barnesHutTheta is the opening criterion. A cell is approximated as a single
// body when (cell_size / distance) < theta. Lower = more accurate, slower.
// 0.5 is a standard choice balancing accuracy and performance.
const barnesHutTheta = 0.5

// quadNode is one node in a Barnes-Hut quadtree (2D: X/Y plane).
// Each internal node stores aggregate multipole data so we can approximate
// the force from an entire subtree without visiting every particle.
type quadNode struct {
	cx, cy float64 // cell centre
	half   float64 // half-width of the cell

	// Multipole data — accumulated bottom-up after all inserts.
	totalMass float64 // Σ mᵢ
	// netCharge = Σ(mᵢ·qᵢ): positive for matter-dominated cells, negative for
	// antimatter-dominated. This is what makes Barnes-Hut correct for our
	// conjecture. The force on particle p from a cell reduces to:
	//   F = G · m_p · q_p · netCharge / r²
	// which gives the right sign (attract or repel) without visiting every particle.
	netCharge float64
	cmx, cmy  float64 // centre of mass (mass-weighted position)

	children [4]*quadNode
	particle *Particle // non-nil only for a leaf holding exactly one particle
}

// buildTree constructs a quadtree from the given particles and returns the root.
// All particles must already be wrapped into [-boxSize/2, boxSize/2).
func buildTree(particles []Particle, boxSize float64) *quadNode {
	root := &quadNode{cx: 0, cy: 0, half: boxSize / 2}
	for i := range particles {
		root.insert(&particles[i])
	}
	root.computeMass()
	return root
}

// insert adds p into the subtree rooted at n.
func (n *quadNode) insert(p *Particle) {
	if n.particle == nil && n.children[0] == nil {
		// Empty leaf — claim it.
		n.particle = p
		return
	}

	if n.particle != nil {
		// Occupied leaf — subdivide and push the existing particle down.
		if n.half < 1e-6 {
			// Particles are coincident to floating-point precision.
			// Softening handles the near-zero force; skip insertion.
			return
		}
		existing := n.particle
		n.particle = nil
		n.subdivide()
		n.insertChild(existing)
	}
	n.insertChild(p)
}

func (n *quadNode) subdivide() {
	quarter := n.half / 2
	for q := 0; q < 4; q++ {
		cx := n.cx - quarter
		if q&1 != 0 {
			cx = n.cx + quarter
		}
		cy := n.cy - quarter
		if q&2 != 0 {
			cy = n.cy + quarter
		}
		n.children[q] = &quadNode{cx: cx, cy: cy, half: quarter}
	}
}

func (n *quadNode) insertChild(p *Particle) {
	q := 0
	if p.Position.X >= n.cx {
		q |= 1
	}
	if p.Position.Y >= n.cy {
		q |= 2
	}
	n.children[q].insert(p)
}

// computeMass accumulates totalMass, netCharge, and centre-of-mass bottom-up.
// Must be called once after all inserts, before any force queries.
func (n *quadNode) computeMass() {
	if n.particle != nil {
		n.totalMass = n.particle.Mass
		n.netCharge = n.particle.Mass * n.particle.GCharge
		n.cmx = n.particle.Position.X
		n.cmy = n.particle.Position.Y
		return
	}
	var mwX, mwY float64
	for _, child := range n.children {
		if child == nil {
			continue
		}
		child.computeMass()
		n.totalMass += child.totalMass
		n.netCharge += child.netCharge
		mwX += child.totalMass * child.cmx
		mwY += child.totalMass * child.cmy
	}
	if n.totalMass > 0 {
		n.cmx = mwX / n.totalMass
		n.cmy = mwY / n.totalMass
	}
}

// force returns the net gravitational force on particle p from this subtree.
func (n *quadNode) force(p *Particle, boxSize float64) Vector3 {
	if n.totalMass == 0 || n.particle == p {
		return Vector3{}
	}

	// Displacement to cell centre-of-mass, using minimum image for the torus.
	dx := minImageScalar(n.cmx-p.Position.X, boxSize)
	dy := minImageScalar(n.cmy-p.Position.Y, boxSize)
	distSq := dx*dx + dy*dy
	dist := math.Sqrt(distSq)

	// Leaf nodes are always treated as a single body (they are one particle).
	// Internal nodes use the BH criterion: approximate when far enough away.
	useCell := n.particle != nil || (dist > 0 && (2*n.half)/dist < barnesHutTheta)

	if useCell {
		if distSq < Softening*Softening {
			distSq = Softening * Softening
			dist = math.Sqrt(distSq)
		}
		// netCharge carries the sign: positive = matter-dominant (attracts matter,
		// repels antimatter), negative = antimatter-dominant (vice versa).
		forceMag := G * p.Mass * p.GCharge * n.netCharge / distSq
		return Vector3{X: dx * forceMag / dist, Y: dy * forceMag / dist}
	}

	// Cell is too close — recurse into children.
	var total Vector3
	for _, child := range n.children {
		if child != nil {
			total = total.Add(child.force(p, boxSize))
		}
	}
	return total
}
