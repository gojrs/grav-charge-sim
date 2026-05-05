// localserver runs a 3D gravitational-charge simulation server-side and
// streams particle state to a Three.js browser client over SSE.
// Local development only — not deployed.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gojrs/grav-charge-sim/physics"
)

//go:embed client
var clientFiles embed.FS

var (
	simMu   sync.RWMutex
	sim     *physics.Simulation
	simCfg  physics.Config
	workers = runtime.NumCPU()
)

func main() {
	workers = runtime.NumCPU()
	simCfg = physics.Config{
		N:                500,
		BoxSize:          800,
		MaxSpeed:         10,
		ParticleMass:     1.0,
		ThreeDimensional: true,
	}
	sim = physics.New(simCfg)

	sub, err := fs.Sub(clientFiles, "client")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(sub)))
	http.HandleFunc("/sim/stream", handleStream)
	http.HandleFunc("/sim/reset", handleReset)
	http.HandleFunc("/env", handleEnv)

	// Physics loop — step as fast as target dt allows; workers parallelize forces.
	dt := 0.016
	go func() {
		for {
			start := time.Now()
			simMu.Lock()
			sim.Step3D(dt, workers)
			simMu.Unlock()
			if elapsed := time.Since(start); elapsed < 16*time.Millisecond {
				time.Sleep(16*time.Millisecond - elapsed)
			}
		}
	}()

	port := "8089"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Printf("local 3D server at http://localhost:%s  (Ctrl-C to stop)", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// frame is the JSON payload sent per SSE tick.
type frame struct {
	P   []float32 `json:"p"`             // [x,y,z,charge,mass, ...] stride 5
	F   []float32 `json:"f,omitempty"`   // [fx,fy,fz, ...] stride 3 — net force per particle
	Fab []float32 `json:"fab,omitempty"` // [ax,ay,az,bx,by,bz,kind, ...] stride 7 — fabric pairs
	S   struct {
		Matter int `json:"matter"`
		Anti   int `json:"anti"`
		Steps  int `json:"steps"`
		Ann    int `json:"ann"`
		Merged int `json:"merged"`
	} `json:"s"`
}

type snapshotOpts struct {
	forces bool
	fabric bool
}

// snapshot copies the current simulation state under a read lock and encodes it.
func snapshot(opts snapshotOpts) []byte {
	simMu.RLock()
	particles := make([]physics.Particle, len(sim.Particles))
	copy(particles, sim.Particles)
	steps  := sim.StepCount
	ann    := sim.AnnihilationCount
	merged := sim.MergeCount
	var forces []physics.Vector3
	var pairs  []physics.ForcePair
	if opts.forces {
		forces = sim.ComputeForces3D()
	}
	if opts.fabric {
		pairs = sim.ComputeFabricPairs(0.0005)
	}
	simMu.RUnlock()

	var f frame
	f.P = make([]float32, len(particles)*5)
	for i, p := range particles {
		f.P[i*5]   = float32(p.Position.X)
		f.P[i*5+1] = float32(p.Position.Y)
		f.P[i*5+2] = float32(p.Position.Z)
		f.P[i*5+3] = float32(p.GCharge)
		f.P[i*5+4] = float32(p.Mass)
		if p.IsMatter() {
			f.S.Matter++
		} else {
			f.S.Anti++
		}
	}
	f.S.Steps  = steps
	f.S.Ann    = ann
	f.S.Merged = merged

	if opts.forces && len(forces) > 0 {
		f.F = make([]float32, len(forces)*3)
		for i, v := range forces {
			f.F[i*3]   = float32(v.X)
			f.F[i*3+1] = float32(v.Y)
			f.F[i*3+2] = float32(v.Z)
		}
	}
	if opts.fabric && len(pairs) > 0 {
		f.Fab = make([]float32, len(pairs)*7)
		for i, p := range pairs {
			f.Fab[i*7]   = float32(p.AX)
			f.Fab[i*7+1] = float32(p.AY)
			f.Fab[i*7+2] = float32(p.AZ)
			f.Fab[i*7+3] = float32(p.BX)
			f.Fab[i*7+4] = float32(p.BY)
			f.Fab[i*7+5] = float32(p.BZ)
			f.Fab[i*7+6] = float32(p.Kind)
		}
	}

	data, _ := json.Marshal(f)
	return data
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	opts := snapshotOpts{
		forces: q.Get("forces") == "1",
		fabric: q.Get("fabric") == "1",
	}

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "data: %s\n\n", snapshot(opts))
			flusher.Flush()
		}
	}
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	n := simCfg.N
	if _, err := fmt.Sscan(r.URL.Query().Get("n"), &n); err == nil && n > 0 {
		simCfg.N = n
	}
	simMu.Lock()
	sim = physics.New(simCfg)
	simMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func handleEnv(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"type":    "local",
		"workers": workers,
		"warn": map[string]int{
			"forceLines":  1000,
			"splitForces": 500,
			"fabric":      500,
		},
	})
}
