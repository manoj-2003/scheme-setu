package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// Result is the discovery output.
type Result struct {
	Leads []models.Lead `json:"leads"`
	// Questions applicants actually type, from Google Autocomplete.
	Questions []string `json:"questions,omitempty"`
	Queries   []string `json:"queries,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Options configures a discovery run.
type Options struct {
	Languages []string
	// MaxQueries caps SerpApi calls for one run. Keep this small.
	MaxQueries int
	// MaxLeads caps how many leads are returned.
	MaxLeads int
	// KnownKeywords are catalog scheme names and keywords. Leads matching
	// these are dropped, so the "discovered" list only holds genuinely new
	// finds rather than re-showing MUDRA five times.
	KnownKeywords []string
	// Concurrency limits parallel SerpApi calls.
	Concurrency int
}

func (o *Options) applyDefaults() {
	if len(o.Languages) == 0 {
		o.Languages = []string{"en"}
	}
	if o.MaxQueries == 0 {
		o.MaxQueries = 6
	}
	if o.MaxLeads == 0 {
		o.MaxLeads = 12
	}
	if o.Concurrency == 0 {
		o.Concurrency = 4
	}
}

// Run executes the query plan for a profile and returns deduplicated,
// confidence-scored leads.
//
// Partial failure is normal and expected: in cache-only mode most queries
// miss, and a single bad query should never sink the request. Every failure
// becomes a warning and the rest of the fan-out continues.
func Run(ctx context.Context, client *serp.Client, p models.Profile, opts Options) Result {
	opts.applyDefaults()

	plan := Plan(p, languagesFor(p, opts.Languages), opts.MaxQueries)

	outcomes := make([]queryOutcome, len(plan))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	for i, q := range plan {
		g.Go(func() error {
			out := queryOutcome{query: q, queryString: q.Params.Query()}

			resp, err := client.Search(ctx, q.Params)
			if err != nil {
				// A cache miss in offline mode is expected, not an error
				// worth shouting about.
				if errors.Is(err, serp.ErrCacheMiss) {
					out.warning = fmt.Sprintf("not cached: %s [%s]", q.Label, q.Language)
				} else {
					out.warning = fmt.Sprintf("%s [%s]: %v", q.Label, q.Language, err)
				}
				outcomes[i] = out
				return nil
			}

			for _, s := range resp.Suggestions {
				if v := strings.TrimSpace(s.Value); v != "" {
					out.questions = append(out.questions, v)
				}
			}

			for _, r := range resp.Organic {
				lead, ok := toLead(r, q)
				if !ok {
					continue
				}
				out.leads = append(out.leads, lead)
			}

			outcomes[i] = out
			return nil
		})
	}
	// No goroutine returns an error, so this only surfaces context
	// cancellation.
	_ = g.Wait()

	return collect(outcomes, opts)
}

// queryOutcome is the per-query result of the discovery fan-out. Each slot is
// written by exactly one goroutine, so no mutex is needed.
type queryOutcome struct {
	query       Query
	leads       []models.Lead
	questions   []string
	warning     string
	queryString string
}

func collect(outcomes []queryOutcome, opts Options) Result {
	var res Result

	seenLink := map[string]bool{}
	seenQuestion := map[string]bool{}

	for _, o := range outcomes {
		if o.queryString != "" {
			res.Queries = append(res.Queries, o.queryString)
		}
		if o.warning != "" {
			res.Warnings = append(res.Warnings, o.warning)
		}

		for _, q := range o.questions {
			key := strings.ToLower(q)
			if !seenQuestion[key] {
				seenQuestion[key] = true
				res.Questions = append(res.Questions, q)
			}
		}

		for _, lead := range o.leads {
			if seenLink[lead.Link] {
				continue
			}
			if isKnown(lead, opts.KnownKeywords) {
				continue
			}
			seenLink[lead.Link] = true
			res.Leads = append(res.Leads, lead)
		}
	}

	sort.SliceStable(res.Leads, func(i, j int) bool {
		return res.Leads[i].Confidence > res.Leads[j].Confidence
	})
	if len(res.Leads) > opts.MaxLeads {
		res.Leads = res.Leads[:opts.MaxLeads]
	}
	if len(res.Questions) > 8 {
		res.Questions = res.Questions[:8]
	}
	return res
}

// toLead converts an organic result into a lead, discarding anything that is
// not plausibly a government scheme page.
func toLead(r serp.OrganicResult, q Query) (models.Lead, bool) {
	domain := Domain(r.Link)
	if domain == "" {
		return models.Lead{}, false
	}

	official := IsGovDomain(domain)
	if !official {
		// Discovery only proposes official sources. Unofficial pages ranking
		// for scheme names are handled by the fraud shield, not surfaced as
		// leads.
		return models.Lead{}, false
	}

	text := strings.ToLower(r.Title + " " + r.Snippet)
	if !looksLikeScheme(text) {
		return models.Lead{}, false
	}

	return models.Lead{
		Title:      strings.TrimSpace(r.Title),
		Link:       r.Link,
		Snippet:    strings.TrimSpace(r.Snippet),
		Domain:     domain,
		Query:      q.Params.Query(),
		Language:   q.Language,
		Official:   true,
		Confidence: confidence(r, q, text),
	}, true
}

var schemeSignals = []string{
	"scheme", "yojana", "yojna", "subsidy", "loan", "grant", "assistance",
	"incentive", "corporation", "mission", "guidelines", "योजना", "अनुदान",
	"திட்டம்", "প্রকল্প", "పథకం", "ಯೋಜನೆ",
}

func looksLikeScheme(lowerText string) bool {
	for _, sig := range schemeSignals {
		if strings.Contains(lowerText, sig) {
			return true
		}
	}
	return false
}

func confidence(r serp.OrganicResult, q Query, lowerText string) float64 {
	c := 0.4 * q.Weight

	// Rank position is a real signal here: government portals rank well for
	// their own scheme names.
	switch {
	case r.Position <= 3:
		c += 0.3
	case r.Position <= 6:
		c += 0.2
	default:
		c += 0.1
	}

	if strings.Contains(lowerText, "apply") || strings.Contains(lowerText, "online") {
		c += 0.1
	}
	if strings.Contains(lowerText, "eligib") {
		c += 0.1
	}
	if strings.HasSuffix(strings.ToLower(r.Link), ".pdf") {
		// A guidelines PDF is authoritative but a bad landing page for an
		// applicant, so it ranks slightly below an HTML page.
		c -= 0.05
	}

	if c > 1 {
		c = 1
	}
	return c
}

func isKnown(lead models.Lead, known []string) bool {
	text := strings.ToLower(lead.Title + " " + lead.Snippet)
	for _, k := range known {
		if k == "" {
			continue
		}
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

// Domain extracts a bare hostname from a URL, dropping any www prefix.
func Domain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// IsGovDomain reports whether a host is an Indian government domain.
func IsGovDomain(host string) bool {
	host = strings.ToLower(host)
	return host == "gov.in" || host == "nic.in" ||
		strings.HasSuffix(host, ".gov.in") ||
		strings.HasSuffix(host, ".nic.in")
}

// languagesFor decides which hl= values a run searches in.
//
// The applicant's own language leads, because that is the one that surfaces a
// state circular. English follows as the fallback, since most portals publish
// at least a stub in English and it is what the catalog keywords match on.
//
// Crucially the configured list is *replaced*, not extended: searching Hindi
// for a Tamil applicant spends credits on pages Tamil Nadu never published,
// and because Plan truncates to MaxQueries, every irrelevant language pushes
// out a high-value query (the filetype:pdf circular search is the first to
// go). The configured list still applies to profiles that name no language,
// which is how the demo personas and cmd/warm get their breadth.
func languagesFor(p models.Profile, configured []string) []string {
	lang := p.Lang()
	if lang == "" || strings.EqualFold(lang, "en") {
		return configured
	}
	return []string{lang, "en"}
}
