// Package serp is a credit-aware SerpApi client: every response is cached,
// every call is counted, and the whole thing can run with the network turned
// off so a demo never depends on a live quota.
package serp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Endpoint is the SerpApi search endpoint.
const Endpoint = "https://serpapi.com/search.json"

// Engine identifiers used by this project.
const (
	EngineGoogle             = "google"
	EngineGoogleNews         = "google_news"
	EngineGoogleAutocomplete = "google_autocomplete"
	EngineGoogleMaps         = "google_maps"
)

var (
	// ErrCacheMiss is returned in cache-only mode when a query has never
	// been fetched. It is not a failure: callers degrade gracefully and note
	// it as a warning.
	ErrCacheMiss = errors.New("serp: cache miss and client is in cache-only mode")

	// ErrCreditBudget is returned once the per-process call ceiling is hit.
	// This is the guard that stops a runaway loop from eating a month of
	// free-tier credits.
	ErrCreditBudget = errors.New("serp: credit budget for this process exhausted")

	// ErrNoAPIKey is returned when a live call is attempted without a key.
	ErrNoAPIKey = errors.New("serp: SERPAPI_KEY is not set")
)

// Params is a SerpApi request. Keys are the raw API parameter names, so the
// docs read across directly.
type Params map[string]string

// Engine returns the engine parameter, defaulting to plain Google search.
func (p Params) Engine() string {
	if e := p["engine"]; e != "" {
		return e
	}
	return EngineGoogle
}

// Query returns the q parameter.
func (p Params) Query() string { return p["q"] }

// Redacted copies the params with the API key removed, for logging and for
// storing alongside cache entries.
func (p Params) Redacted() Params {
	out := make(Params, len(p))
	for k, v := range p {
		if strings.EqualFold(k, "api_key") {
			continue
		}
		out[k] = v
	}
	return out
}

func (p Params) encode() string {
	v := url.Values{}
	for key, val := range p {
		v.Set(key, val)
	}
	return v.Encode()
}

// Mode controls whether the client is allowed to spend credits.
type Mode string

const (
	// ModeLive fetches on a cache miss and stores the result.
	ModeLive Mode = "live"
	// ModeCacheOnly never touches the network. Use it for the demo
	// recording and for clones that have no API key.
	ModeCacheOnly Mode = "cache"
)

// Options configures a Client.
type Options struct {
	APIKey      string
	Mode        Mode
	CachePath   string
	CacheTTL    time.Duration
	MaxCredits  int
	HTTPTimeout time.Duration
}

func (o *Options) applyDefaults() {
	if o.Mode == "" {
		o.Mode = ModeLive
	}
	if o.CachePath == "" {
		o.CachePath = "data/serp-cache.sqlite"
	}
	if o.CacheTTL == 0 {
		o.CacheTTL = 7 * 24 * time.Hour
	}
	if o.MaxCredits == 0 {
		o.MaxCredits = 40
	}
	if o.HTTPTimeout == 0 {
		o.HTTPTimeout = 20 * time.Second
	}
	if o.APIKey == "" {
		// Without a key there is nothing to spend, so fall back to serving
		// whatever is already cached rather than failing every request.
		o.Mode = ModeCacheOnly
	}
}

// Client is a caching SerpApi client. It is safe for concurrent use, which
// matters because discovery fans out across languages with an errgroup.
type Client struct {
	opts  Options
	http  *http.Client
	cache *Cache

	mu        sync.Mutex
	liveCalls int
	cacheHits int
	calls     int
}

// New builds a Client and opens its cache.
func New(opts Options) (*Client, error) {
	opts.applyDefaults()

	cache, err := OpenCache(opts.CachePath, opts.CacheTTL)
	if err != nil {
		return nil, err
	}

	return &Client{
		opts:  opts,
		cache: cache,
		http:  &http.Client{Timeout: opts.HTTPTimeout},
	}, nil
}

// Close releases the cache handle.
func (c *Client) Close() error { return c.cache.Close() }

// Mode reports the client's current mode.
func (c *Client) Mode() Mode { return c.opts.Mode }

// CachedResponses returns how many responses are on disk.
func (c *Client) CachedResponses(ctx context.Context) int { return c.cache.Count(ctx) }

// Usage snapshots the counters for this process.
func (c *Client) Usage() (calls, cacheHits, liveCalls, maxCredits int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.cacheHits, c.liveCalls, c.opts.MaxCredits
}

// Response is a SerpApi result with the handful of fields this project uses
// decoded, plus the raw payload for anything else.
type Response struct {
	Organic     []OrganicResult `json:"organic_results"`
	News        []NewsResult    `json:"news_results"`
	Suggestions []Suggestion    `json:"suggestions"`
	Error       string          `json:"error"`

	Raw       json.RawMessage `json:"-"`
	FromCache bool            `json:"-"`
	FetchedAt time.Time       `json:"-"`
}

// OrganicResult is one Google organic hit.
type OrganicResult struct {
	Position     int    `json:"position"`
	Title        string `json:"title"`
	Link         string `json:"link"`
	DisplayedURL string `json:"displayed_link"`
	Snippet      string `json:"snippet"`
	Date         string `json:"date"`
	Source       string `json:"source"`
}

// NewsResult is one Google News hit.
type NewsResult struct {
	Position  int    `json:"position"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Snippet   string `json:"snippet"`
	Date      string `json:"date"`
	Source    any    `json:"source"`
	Thumbnail string `json:"thumbnail"`
}

// SourceName flattens the News source field, which SerpApi returns either as
// a plain string or as an object depending on the result.
func (n NewsResult) SourceName() string {
	switch v := n.Source.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return ""
}

// Suggestion is one Google Autocomplete entry.
type Suggestion struct {
	Value string `json:"value"`
}

// Search runs a query, serving from cache when possible. In live mode a miss
// costs one credit and the result is cached for next time.
func (c *Client) Search(ctx context.Context, p Params) (*Response, error) {
	key := Key(p)

	c.mu.Lock()
	c.calls++
	c.mu.Unlock()

	if payload, fetchedAt, ok := c.cache.Get(ctx, key); ok {
		c.mu.Lock()
		c.cacheHits++
		c.mu.Unlock()

		resp, err := decode(payload)
		if err != nil {
			return nil, fmt.Errorf("decode cached response: %w", err)
		}
		resp.FromCache = true
		resp.FetchedAt = fetchedAt
		return resp, nil
	}

	if c.opts.Mode == ModeCacheOnly {
		return nil, fmt.Errorf("%w (engine=%s q=%q)", ErrCacheMiss, p.Engine(), p.Query())
	}
	if c.opts.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	if err := c.reserveCredit(); err != nil {
		return nil, err
	}

	payload, err := c.fetch(ctx, p)
	if err != nil {
		return nil, err
	}

	resp, err := decode(payload)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	// SerpApi returns HTTP 200 with an "error" field for things like "no
	// results found". Cache those too -- they cost a credit either way.
	if err := c.cache.Put(ctx, key, p, payload); err != nil {
		return nil, err
	}

	resp.FetchedAt = time.Now()
	return resp, nil
}

func (c *Client) reserveCredit() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.liveCalls >= c.opts.MaxCredits {
		return fmt.Errorf("%w (limit %d)", ErrCreditBudget, c.opts.MaxCredits)
	}
	c.liveCalls++
	return nil
}

func (c *Client) fetch(ctx context.Context, p Params) ([]byte, error) {
	q := p.Redacted()
	q["api_key"] = c.opts.APIKey
	q["output"] = "json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Endpoint+"?"+q.encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serpapi request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		// Surface SerpApi's own message, which is far more useful than the
		// status code alone (bad key, out of credits, malformed param).
		var e struct{ Error string }
		_ = json.Unmarshal(body, &e)
		if e.Error != "" {
			return nil, fmt.Errorf("serpapi %d: %s", res.StatusCode, e.Error)
		}
		return nil, fmt.Errorf("serpapi %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func decode(payload []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	resp.Raw = json.RawMessage(payload)
	return &resp, nil
}
