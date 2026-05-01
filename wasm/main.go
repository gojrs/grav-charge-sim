//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/gojrs/grav-charge-sim/physics"
)

var sim *physics.Simulation

// initSim(n int) — called from JS to (re)start the simulation.
// Passing n <= 0 uses the default particle count.
func initSim(_ js.Value, args []js.Value) any {
	cfg := physics.DefaultConfig()
	if len(args) > 0 && args[0].Int() > 0 {
		cfg.N = args[0].Int()
	}
	sim = physics.New(cfg)
	return nil
}

// step(dt float64) — advance the simulation by dt.
// Called once per animation frame from JS.
func step(_ js.Value, args []js.Value) any {
	if sim == nil {
		return nil
	}
	dt := 0.016 // ~60 fps default
	if len(args) > 0 {
		dt = args[0].Float()
	}
	sim.Step(dt)
	return nil
}

// getParticles() — returns a flat JS Float64Array: [x, y, gcharge, mass, ...]
// Stride 4. gcharge sign picks colour; mass drives rendered dot size.
func getParticles(_ js.Value, _ []js.Value) any {
	if sim == nil {
		return js.Global().Get("Float64Array").New(0)
	}
	n := len(sim.Particles)
	buf := js.Global().Get("Float64Array").New(n * 4)
	for i, p := range sim.Particles {
		buf.SetIndex(i*4+0, p.Position.X)
		buf.SetIndex(i*4+1, p.Position.Y)
		buf.SetIndex(i*4+2, p.GCharge)
		buf.SetIndex(i*4+3, p.Mass)
	}
	return buf
}

// getSplitForces() — Float64Array stride 4: [ax,ay,rx,ry per particle]
// Attractive (same-charge) and repulsive (opposite-charge) components, computed pairwise.
func getSplitForces(_ js.Value, _ []js.Value) any {
	if sim == nil {
		return js.Global().Get("Float64Array").New(0)
	}
	attr, rep := sim.ComputeSplitForces()
	n := len(attr)
	buf := js.Global().Get("Float64Array").New(n * 4)
	for i := range attr {
		buf.SetIndex(i*4+0, attr[i].X)
		buf.SetIndex(i*4+1, attr[i].Y)
		buf.SetIndex(i*4+2, rep[i].X)
		buf.SetIndex(i*4+3, rep[i].Y)
	}
	return buf
}

// getFabricPairs(threshold?) — Float64Array stride 5: [x1,y1,x2,y2,kind, ...]
// kind: 1=matter-matter, 2=anti-anti, 3=matter-anti. Default threshold 0.003.
func getFabricPairs(_ js.Value, args []js.Value) any {
	if sim == nil {
		return js.Global().Get("Float64Array").New(0)
	}
	threshold := 0.003
	if len(args) > 0 {
		threshold = args[0].Float()
	}
	pairs := sim.ComputeFabricPairs(threshold)
	buf := js.Global().Get("Float64Array").New(len(pairs) * 5)
	for i, p := range pairs {
		buf.SetIndex(i*5+0, p.AX)
		buf.SetIndex(i*5+1, p.AY)
		buf.SetIndex(i*5+2, p.BX)
		buf.SetIndex(i*5+3, p.BY)
		buf.SetIndex(i*5+4, p.Kind)
	}
	return buf
}

// getForces() — returns a flat JS Float64Array: [fx, fy, fx, fy, ...]
// Stride 2. Indices align with getParticles() particle order.
// Forces are computed from current particle positions without advancing the sim.
func getForces(_ js.Value, _ []js.Value) any {
	if sim == nil {
		return js.Global().Get("Float64Array").New(0)
	}
	forces := sim.ComputeForces()
	buf := js.Global().Get("Float64Array").New(len(forces) * 2)
	for i, f := range forces {
		buf.SetIndex(i*2+0, f.X)
		buf.SetIndex(i*2+1, f.Y)
	}
	return buf
}

// getStats() — returns a plain JS object with running counters.
func getStats(_ js.Value, _ []js.Value) any {
	if sim == nil {
		return js.ValueOf(map[string]any{
			"particles":    0,
			"annihilated":  0,
			"steps":        0,
		})
	}
	matter, antimatter := 0, 0
	for _, p := range sim.Particles {
		if p.IsMatter() {
			matter++
		} else {
			antimatter++
		}
	}
	return js.ValueOf(map[string]any{
		"particles":   len(sim.Particles),
		"matter":      matter,
		"antimatter":  antimatter,
		"annihilated": sim.AnnihilationCount,
		"merged":      sim.MergeCount,
		"steps":       sim.StepCount,
	})
}

func main() {
	js.Global().Set("gravSim", js.ValueOf(map[string]any{
		"init":           js.FuncOf(initSim),
		"step":           js.FuncOf(step),
		"getParticles":   js.FuncOf(getParticles),
		"getForces":      js.FuncOf(getForces),
		"getSplitForces": js.FuncOf(getSplitForces),
		"getFabricPairs": js.FuncOf(getFabricPairs),
		"getStats":       js.FuncOf(getStats),
	}))

	// Keep the WASM module alive — the runtime requires main() to block.
	select {}
}
