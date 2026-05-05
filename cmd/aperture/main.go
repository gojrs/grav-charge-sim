package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

//go:embed index.html main.js
var staticFiles embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/run", handleRun)
	mux.Handle("/", http.FileServer(http.FS(staticFiles)))
	log.Printf("aperture at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	cfg := DefaultShotConfig()
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "bad JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.NumShots <= 0 || cfg.NumShots > 500000 {
		cfg.NumShots = 10000
	}
	if cfg.StepsPerShot <= 0 {
		cfg.StepsPerShot = 1
	}
	if cfg.ApertureHalfW <= 0 {
		cfg.ApertureHalfW = 2.5
	}
	if cfg.WallX <= 0 {
		cfg.WallX = 50
	}
	if cfg.AnchorDistY <= 0 {
		cfg.AnchorDistY = 30
	}
	if cfg.AnchorMass <= 0 {
		cfg.AnchorMass = 500
	}
	if len(cfg.EmitterYs) == 0 {
		cfg.EmitterYs = []float64{-cfg.ApertureHalfW, cfg.ApertureHalfW}
	}
	if cfg.EmitterAnchorDist <= 0 {
		cfg.EmitterAnchorDist = 15
	}
	if cfg.EmitterAnchorMass <= 0 {
		cfg.EmitterAnchorMass = 500
	}
	if cfg.FalloffRate <= 0 {
		cfg.FalloffRate = 1.0
	}
	result := RunShots(cfg)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("encode error: %v", err)
	}
}
