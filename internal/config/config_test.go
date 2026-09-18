package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A judge cloning the repo has no key and will very likely just run
// `go run ./cmd/server` rather than the documented cache-mode invocation.
// That path has to land on the committed fixture, because the alternative --
// an empty data/serp-cache.sqlite -- turns every discovery, freshness and
// fraud lookup into a cache miss and makes a working offline demo look broken.
func TestDefaultCachePathWithoutKeyPrefersFixture(t *testing.T) {
	withFixture(t)

	if got := defaultCachePath(""); got != fixtureCachePath {
		t.Errorf("defaultCachePath(\"\") = %q, want %q", got, fixtureCachePath)
	}
}

// With a key the server fetches and stores its own responses, which must not
// be written into the committed fixture.
func TestDefaultCachePathWithKeyUsesDataDir(t *testing.T) {
	withFixture(t)

	if got := defaultCachePath("some-key"); got != liveCachePath {
		t.Errorf("defaultCachePath(key) = %q, want %q", got, liveCachePath)
	}
}

// Started from outside the repo root there is no fixture to serve, and we
// would rather fall back than create an empty fixtures/ directory.
func TestDefaultCachePathWithoutFixtureFallsBack(t *testing.T) {
	t.Chdir(t.TempDir())

	if got := defaultCachePath(""); got != liveCachePath {
		t.Errorf("defaultCachePath(\"\") = %q, want %q", got, liveCachePath)
	}
}

// An explicit SERP_CACHE_PATH always wins, so the Makefile and the README can
// keep naming the fixture outright.
func TestLoadHonoursExplicitCachePath(t *testing.T) {
	withFixture(t)
	t.Setenv("SERPAPI_KEY", "")
	t.Setenv("SERP_CACHE_PATH", "somewhere/else.sqlite")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SerpCachePath != "somewhere/else.sqlite" {
		t.Errorf("SerpCachePath = %q, want the explicit value", cfg.SerpCachePath)
	}
}

// withFixture runs the test in a scratch directory holding a stand-in for the
// committed cache, so the assertions do not depend on the real fixture.
func withFixture(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(fixtureCachePath)), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fixtureCachePath), nil, 0o644); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	t.Chdir(dir)
}
