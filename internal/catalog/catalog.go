// Package catalog loads the curated scheme rule set.
//
// Why a curated catalog at all, in a search-data project: eligibility for a
// loan is a structured rules problem, and inferring "you qualify for Rs 10
// lakh" from a search snippet is how you give someone wrong advice about
// money. So the catalog holds the rules, and SerpApi does the two things a
// static file cannot -- find the state schemes nobody has indexed, and tell
// us whether what we hold is still true today.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/manoj-2003/scheme-setu/internal/models"
)

//go:embed schemes.json
var embedded embed.FS

// Catalog is an in-memory, read-only set of schemes.
type Catalog struct {
	schemes []models.Scheme
	byID    map[string]models.Scheme
}

// Load reads the catalog compiled into the binary.
func Load() (*Catalog, error) {
	b, err := embedded.ReadFile("schemes.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded catalog: %w", err)
	}
	return parse(b)
}

// LoadFile reads a catalog from disk, for editing the rule set without a
// rebuild.
func LoadFile(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog %s: %w", path, err)
	}
	return parse(b)
}

func parse(b []byte) (*Catalog, error) {
	var schemes []models.Scheme
	if err := json.Unmarshal(b, &schemes); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}

	c := &Catalog{
		schemes: schemes,
		byID:    make(map[string]models.Scheme, len(schemes)),
	}
	for _, s := range schemes {
		if s.ID == "" {
			return nil, fmt.Errorf("catalog entry %q has no id", s.Name)
		}
		if _, dup := c.byID[s.ID]; dup {
			return nil, fmt.Errorf("duplicate catalog id %q", s.ID)
		}
		c.byID[s.ID] = s
	}
	return c, nil
}

// All returns every scheme in the catalog.
func (c *Catalog) All() []models.Scheme { return c.schemes }

// Len returns the catalog size.
func (c *Catalog) Len() int { return len(c.schemes) }

// ByID looks up a single scheme.
func (c *Catalog) ByID(id string) (models.Scheme, bool) {
	s, ok := c.byID[id]
	return s, ok
}

// Candidates cheaply narrows the catalog to schemes worth evaluating for a
// profile: right state, right purpose. This runs before any rule evaluation
// and before any search, so a profile only ever triggers the handful of
// queries that could possibly matter.
func (c *Catalog) Candidates(p models.Profile) []models.Scheme {
	out := make([]models.Scheme, 0, len(c.schemes))
	for _, s := range c.schemes {
		if !s.AppliesToState(p.State) {
			continue
		}
		if !s.ServesPurpose(p.Purpose) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Keywords returns every search keyword and scheme name in the catalog,
// lowercased. Discovery uses this to drop leads for schemes we already know
// about, so the "newly discovered" list only contains genuinely new finds.
func (c *Catalog) Keywords() []string {
	seen := map[string]bool{}
	for _, s := range c.schemes {
		for _, k := range append([]string{s.Name}, s.SearchKeywords...) {
			k = strings.ToLower(strings.TrimSpace(k))
			if k != "" {
				seen[k] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// OfficialDomains returns every domain the catalog trusts, used as the
// baseline allowlist for the fraud shield.
func (c *Catalog) OfficialDomains() []string {
	seen := map[string]bool{}
	for _, s := range c.schemes {
		for _, d := range s.OfficialDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" {
				seen[d] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Unverified returns the IDs of schemes whose amounts and rules have not yet
// been checked against the official source. Do not ship a submission with
// this non-empty.
func (c *Catalog) Unverified() []string {
	var out []string
	for _, s := range c.schemes {
		if s.NeedsVerification {
			out = append(out, s.ID)
		}
	}
	sort.Strings(out)
	return out
}
