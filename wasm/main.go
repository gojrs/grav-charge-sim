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

// getParticles() — returns a flat JS Float64Array: [x,y,gcharge, x,y,gcharge, ...]
// Using 2D (X,Y) for now; Z is ignored by the canvas renderer.
// gcharge is +1.0 (matter) or -1.0 (antimatter) — JS uses the sign to pick colour.
func getParticles(_ js.Value, _ []js.Value) any {
	if sim == nil {
		return js.Global().Get("Float64Array").New(0)
	}
	n := len(sim.Particles)
	buf := js.Global().Get("Float64Array").New(n * 3)
	for i, p := range sim.Particles {
		buf.SetIndex(i*3+0, p.Position.X)
		buf.SetIndex(i*3+1, p.Position.Y)
		buf.SetIndex(i*3+2, p.GCharge)
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
		"steps":       sim.StepCount,
	})
}

func main() {
	js.Global().Set("gravSim", js.ValueOf(map[string]any{
		"init":         js.FuncOf(initSim),
		"step":         js.FuncOf(step),
		"getParticles": js.FuncOf(getParticles),
		"getStats":     js.FuncOf(getStats),
	}))

	// Keep the WASM module alive — the runtime requires main() to block.
	select {}
}
