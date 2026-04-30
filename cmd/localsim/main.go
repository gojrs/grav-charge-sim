// localsim runs the gravitational-charge N-body simulation server-side with
// full Go concurrency. Not deployed — local development and batch experiments only.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gojrs/grav-charge-sim/physics"
)

func main() {
	n := flag.Int("n", 2000, "particle count")
	steps := flag.Int("steps", 0, "steps to run (0 = run until Ctrl-C)")
	dt := flag.Float64("dt", 0.016, "time step per frame")
	workers := flag.Int("workers", runtime.NumCPU(), "goroutines for force calculation")
	interval := flag.Int("interval", 100, "print stats every N steps")
	box := flag.Float64("box", 800, "simulation box size")
	flag.Parse()

	cfg := physics.Config{
		N:            *n,
		BoxSize:      *box,
		MaxSpeed:     10,
		ParticleMass: 1.0,
	}

	sim := physics.New(cfg)

	fmt.Fprintf(os.Stderr, "localsim: n=%d workers=%d dt=%.4f box=%.0f\n",
		*n, *workers, *dt, *box)
	if *steps > 0 {
		fmt.Fprintf(os.Stderr, "running %d steps — Ctrl-C to stop early\n\n", *steps)
	} else {
		fmt.Fprintf(os.Stderr, "running until Ctrl-C\n\n")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		<-stop
		close(done)
	}()

	start := time.Now()

	for step := 1; ; step++ {
		select {
		case <-done:
			fmt.Fprintf(os.Stderr, "\nstopped at step %d  elapsed=%s\n",
				step, time.Since(start).Round(time.Millisecond))
			return
		default:
		}

		sim.StepConcurrent(*dt, *workers)

		if step%*interval == 0 {
			elapsed := time.Since(start)
			sps := float64(step) / elapsed.Seconds()
			m, a := countCharges(sim)
			fmt.Printf("step=%-7d  particles=%-5d  matter=%-5d  anti=%-5d  ann=%-5d  merged=%-5d  %.1f steps/s\n",
				step, len(sim.Particles), m, a,
				sim.AnnihilationCount, sim.MergeCount, sps)
		}

		if *steps > 0 && step >= *steps {
			elapsed := time.Since(start)
			m, a := countCharges(sim)
			fmt.Printf("\nfinal: particles=%d  matter=%d  anti=%d  ann=%d  merged=%d\n",
				len(sim.Particles), m, a, sim.AnnihilationCount, sim.MergeCount)
			fmt.Fprintf(os.Stderr, "completed %d steps in %s  (%.1f steps/s)\n",
				step, elapsed.Round(time.Millisecond), float64(step)/elapsed.Seconds())
			return
		}
	}
}

func countCharges(sim *physics.Simulation) (matter, anti int) {
	for _, p := range sim.Particles {
		if p.IsMatter() {
			matter++
		} else {
			anti++
		}
	}
	return
}