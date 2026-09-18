// Package config loads runtime settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Port        string
	CORSOrigins []string

	SerpAPIKey     string
	SerpMode       serp.Mode
	SerpMaxCredits int
	SerpCachePath  string
	SerpCacheTTL   time.Duration

	DiscoveryLangs      []string
	MaxDiscoveryQueries int
	VerifyTopN          int

	// CatalogPath overrides the embedded catalog, for editing rules without
	// rebuilding the binary.
	CatalogPath string

	// WebDir is an optional directory of built frontend assets to serve at /,
	// so a judge can run the whole thing from one binary.
	WebDir string
}

const (
	// liveCachePath is where a keyed run accumulates its own responses.
	liveCachePath = "data/serp-cache.sqlite"
	// fixtureCachePath is the warmed demo cache committed to the repo.
	fixtureCachePath = "fixtures/serp-cache.sqlite"
)

// Load reads .env (if present) and then the environment.
func Load() (Config, error) {
	// A missing .env is normal: in cache-only mode the app needs no secrets.
	_ = godotenv.Load()

	key := strings.TrimSpace(os.Getenv("SERPAPI_KEY"))

	cfg := Config{
		Port:                env("PORT", "8080"),
		CORSOrigins:         splitCSV(env("CORS_ORIGINS", "http://localhost:3000")),
		SerpAPIKey:          key,
		SerpMode:            serp.Mode(strings.ToLower(env("SERP_MODE", "live"))),
		SerpCachePath:       env("SERP_CACHE_PATH", defaultCachePath(key)),
		DiscoveryLangs:      splitCSV(env("SERP_DISCOVERY_LANGS", "en,hi")),
		CatalogPath:         os.Getenv("CATALOG_PATH"),
		WebDir:              os.Getenv("WEB_DIR"),
		MaxDiscoveryQueries: envInt("SERP_MAX_DISCOVERY_QUERIES", 6),
		VerifyTopN:          envInt("SERP_VERIFY_TOP_N", 3),
		SerpMaxCredits:      envInt("SERP_MAX_CREDITS", 40),
	}

	ttl, err := time.ParseDuration(env("SERP_CACHE_TTL", "168h"))
	if err != nil {
		return Config{}, fmt.Errorf("SERP_CACHE_TTL: %w", err)
	}
	cfg.SerpCacheTTL = ttl

	switch cfg.SerpMode {
	case serp.ModeLive, serp.ModeCacheOnly:
	default:
		return Config{}, fmt.Errorf("SERP_MODE must be %q or %q, got %q",
			serp.ModeLive, serp.ModeCacheOnly, cfg.SerpMode)
	}

	return cfg, nil
}

// SerpOptions maps the config onto the client options.
func (c Config) SerpOptions() serp.Options {
	return serp.Options{
		APIKey:     c.SerpAPIKey,
		Mode:       c.SerpMode,
		CachePath:  c.SerpCachePath,
		CacheTTL:   c.SerpCacheTTL,
		MaxCredits: c.SerpMaxCredits,
	}
}

// defaultCachePath picks the cache a bare `go run ./cmd/server` should use.
// Without a key the client can only serve what is already on disk, so the
// useful default is the committed demo fixture: otherwise a fresh clone opens
// an empty data/serp-cache.sqlite and every discovery, freshness and fraud
// lookup is a miss, which looks like a broken app rather than an offline one.
// The existence check keeps us from creating a stray fixtures/ directory when
// the server is started from outside the repo root.
func defaultCachePath(apiKey string) string {
	if apiKey != "" {
		return liveCachePath
	}
	if _, err := os.Stat(fixtureCachePath); err == nil {
		return fixtureCachePath
	}
	return liveCachePath
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
