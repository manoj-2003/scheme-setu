package serp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	// Pure-Go SQLite driver. Deliberately not mattn/go-sqlite3: that needs
	// CGO and a C toolchain, which is a pointless dependency to impose on
	// anyone cloning this on Windows.
	_ "modernc.org/sqlite"
)

// Cache is a content-addressed store of SerpApi responses keyed by the
// request parameters. It exists because the free plan grants 250 credits a
// month -- roughly eight searches a day -- so every response is worth keeping
// and no query should ever be paid for twice.
type Cache struct {
	db  *sql.DB
	ttl time.Duration
}

const schema = `
CREATE TABLE IF NOT EXISTS serp_cache (
	key        TEXT PRIMARY KEY,
	engine     TEXT NOT NULL,
	query      TEXT NOT NULL,
	params     TEXT NOT NULL,
	payload    BLOB NOT NULL,
	fetched_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_serp_cache_engine ON serp_cache(engine);
`

// OpenCache opens (and creates if needed) the cache at path. A ttl of zero
// means entries never expire, which is what you want when pointing at the
// committed demo fixture.
func OpenCache(path string, ttl time.Duration) (*Cache, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache dir: %w", err)
		}
	}

	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open cache: %w", err)
	}
	// One writer is plenty and avoids SQLITE_BUSY under the discovery fan-out.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate cache: %w", err)
	}
	return &Cache{db: db, ttl: ttl}, nil
}

// Close releases the underlying database handle.
func (c *Cache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Key derives the cache key for a set of request parameters. The API key and
// cache-busting flags are excluded so that the same logical query maps to one
// entry regardless of who ran it.
func Key(p Params) string {
	skip := map[string]bool{"api_key": true, "no_cache": true, "output": true}

	keys := make([]string, 0, len(p))
	for k := range p {
		if skip[strings.ToLower(k)] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s\x00", k, p[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Get returns the cached payload for key. The boolean is false on a miss or
// when the entry is older than the configured TTL.
func (c *Cache) Get(ctx context.Context, key string) (json.RawMessage, time.Time, bool) {
	if c == nil || c.db == nil {
		return nil, time.Time{}, false
	}

	var payload []byte
	var fetchedUnix int64
	err := c.db.QueryRowContext(ctx,
		`SELECT payload, fetched_at FROM serp_cache WHERE key = ?`, key,
	).Scan(&payload, &fetchedUnix)
	if err != nil {
		return nil, time.Time{}, false
	}

	fetchedAt := time.Unix(fetchedUnix, 0)
	if c.ttl > 0 && time.Since(fetchedAt) > c.ttl {
		return nil, fetchedAt, false
	}
	return json.RawMessage(payload), fetchedAt, true
}

// Put stores a response payload against its parameters.
func (c *Cache) Put(ctx context.Context, key string, p Params, payload []byte) error {
	if c == nil || c.db == nil {
		return nil
	}

	paramsJSON, err := json.Marshal(p.Redacted())
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	_, err = c.db.ExecContext(ctx,
		`INSERT INTO serp_cache (key, engine, query, params, payload, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   payload    = excluded.payload,
		   params     = excluded.params,
		   fetched_at = excluded.fetched_at`,
		key, p.Engine(), p.Query(), string(paramsJSON), payload, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

// Count returns how many responses are cached. Used by /api/v1/usage so the
// demo can show it is running offline.
func (c *Cache) Count(ctx context.Context) int {
	if c == nil || c.db == nil {
		return 0
	}
	var n int
	if err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM serp_cache`).Scan(&n); err != nil {
		return 0
	}
	return n
}
