package serp

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// failingTransport fails every request, the way a timeout or a DNS failure
// does. net/http wraps whatever we return here in a *url.Error carrying the
// full request URL.
type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("simulated network failure")
}

// The api_key travels in the query string, so any error that stringifies the
// request URL leaks the key. These errors are collected into the match
// response's "warnings" list and rendered in the browser, so a leak here is a
// leak to every user of the API.
func TestFetchErrorDoesNotLeakAPIKey(t *testing.T) {
	const key = "not-a-real-key-0123456789abcdef"

	client, err := New(Options{
		APIKey:    key,
		Mode:      ModeLive,
		CachePath: filepath.Join(t.TempDir(), "cache.sqlite"),
		Transport: failingTransport{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	_, err = client.Search(context.Background(), Params{
		"engine": EngineGoogle,
		"q":      "pm svanidhi apply online",
		"hl":     "ta",
	})
	if err == nil {
		t.Fatal("expected an error from a failing transport, got nil")
	}

	if strings.Contains(err.Error(), key) {
		t.Errorf("error leaks the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "simulated network failure") {
		t.Errorf("error dropped the underlying reason, leaving nothing to debug: %v", err)
	}
}

// Redacted is what keeps the key out of the cache, which is committed to the
// repo as a demo fixture.
func TestRedactedDropsAPIKey(t *testing.T) {
	p := Params{"engine": EngineGoogle, "q": "test", "api_key": "secret-value"}

	r := p.Redacted()
	if _, ok := r["api_key"]; ok {
		t.Error("Redacted kept api_key")
	}
	if r["q"] != "test" {
		t.Errorf("Redacted dropped a non-secret param: %v", r)
	}
	if _, ok := p["api_key"]; !ok {
		t.Error("Redacted mutated the original Params")
	}
}
