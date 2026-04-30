package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	NtfyTopic            string `json:"ntfy_topic"`
	GitHubToken          string `json:"github_token"`
	ZenodoToken          string `json:"zenodo_token"`
	CheckIntervalMinutes int    `json:"check_interval_minutes"`
}

func loadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.CheckIntervalMinutes <= 0 {
		cfg.CheckIntervalMinutes = 15
	}
	return cfg
}

// ---------------------------------------------------------------------------
// State — persisted between runs so we only notify on changes
// ---------------------------------------------------------------------------

const stateFile = "state.json"

type State struct {
	AskPhysicsComments   int `json:"askphysics_comments"`
	HypotheticalComments int `json:"hypothetical_comments"`
	GitHubStars          int `json:"github_stars"`
	ZenodoViews          int `json:"zenodo_views"`
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{} // first run — all counts start at zero
	}
	var s State
	json.Unmarshal(data, &s)
	return s
}

func saveState(s State) {
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		log.Printf("saveState: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Notifications via ntfy.sh
// ---------------------------------------------------------------------------

func notify(cfg Config, title, message string) {
	req, err := http.NewRequest("POST", "https://ntfy.sh/"+cfg.NtfyTopic, strings.NewReader(message))
	if err != nil {
		log.Printf("notify build request: %v", err)
		return
	}
	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("notify send: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("notified — %s: %s", title, message)
}

// ---------------------------------------------------------------------------
// Reddit
// ---------------------------------------------------------------------------

const (
	redditAskPhysics   = "https://www.reddit.com/r/AskPhysics/comments/1sznufz.json"
	redditHypothetical = "https://www.reddit.com/r/HypotheticalPhysics/comments/1szo24z.json"
)

func fetchRedditComments(url string) (count int, title string, err error) {
	req, _ := http.NewRequest("GET", url, nil)
	// Reddit blocks requests without a User-Agent.
	req.Header.Set("User-Agent", "grav-charge-notifier/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data []struct {
		Data struct {
			Children []struct {
				Data struct {
					Title       string `json:"title"`
					NumComments int    `json:"num_comments"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, "", fmt.Errorf("parse: %w", err)
	}
	if len(data) == 0 || len(data[0].Data.Children) == 0 {
		return 0, "", fmt.Errorf("unexpected response structure")
	}
	post := data[0].Data.Children[0].Data
	return post.NumComments, post.Title, nil
}

// ---------------------------------------------------------------------------
// GitHub
// ---------------------------------------------------------------------------

func fetchGitHubStars(token string) (int, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/gojrs/grav-charge-sim", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data struct {
		Stars int `json:"stargazers_count"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	return data.Stars, nil
}

// ---------------------------------------------------------------------------
// Zenodo
// ---------------------------------------------------------------------------

func fetchZenodoViews(token string) (int, error) {
	req, _ := http.NewRequest("GET", "https://zenodo.org/api/records/19839829", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data struct {
		Stats struct {
			Views float64 `json:"views"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	return int(data.Stats.Views), nil
}

// ---------------------------------------------------------------------------
// Check — runs every interval
// ---------------------------------------------------------------------------

func check(cfg Config, state *State) {
	// Reddit — r/AskPhysics
	if n, title, err := fetchRedditComments(redditAskPhysics); err != nil {
		log.Printf("reddit askphysics: %v", err)
	} else if n > state.AskPhysicsComments {
		notify(cfg, "New comment on r/AskPhysics",
			fmt.Sprintf("%d comments (was %d) — %s", n, state.AskPhysicsComments, title))
		state.AskPhysicsComments = n
	}

	// Reddit — r/HypotheticalPhysics
	if n, title, err := fetchRedditComments(redditHypothetical); err != nil {
		log.Printf("reddit hypothetical: %v", err)
	} else if n > state.HypotheticalComments {
		notify(cfg, "New comment on r/HypotheticalPhysics",
			fmt.Sprintf("%d comments (was %d) — %s", n, state.HypotheticalComments, title))
		state.HypotheticalComments = n
	}

	// GitHub stars
	if stars, err := fetchGitHubStars(cfg.GitHubToken); err != nil {
		log.Printf("github: %v", err)
	} else if stars > state.GitHubStars {
		notify(cfg, "New GitHub star ⭐",
			fmt.Sprintf("grav-charge-sim now has %d stars (was %d)", stars, state.GitHubStars))
		state.GitHubStars = stars
	}

	// Zenodo views
	if views, err := fetchZenodoViews(cfg.ZenodoToken); err != nil {
		log.Printf("zenodo: %v", err)
	} else if views > state.ZenodoViews {
		notify(cfg, "New Zenodo views",
			fmt.Sprintf("Record 19839829 now has %d views (was %d)", views, state.ZenodoViews))
		state.ZenodoViews = views
	}

	saveState(*state)
	log.Printf("check done — r/AskPhysics:%d r/Hypothetical:%d stars:%d zenodo:%d",
		state.AskPhysicsComments, state.HypotheticalComments,
		state.GitHubStars, state.ZenodoViews)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	cfg := loadConfig("config.json")
	state := loadState()

	log.Printf("notifier starting — interval %d min, topic %s", cfg.CheckIntervalMinutes, cfg.NtfyTopic)

	// Run immediately on start, then on each tick.
	check(cfg, &state)

	ticker := time.NewTicker(time.Duration(cfg.CheckIntervalMinutes) * time.Minute)
	for range ticker.C {
		check(cfg, &state)
	}
}
