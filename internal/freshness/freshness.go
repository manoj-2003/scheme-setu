// Package freshness answers the one question a static scheme directory can
// never answer: is this still true today?
//
// Government schemes get extended, revised, paused when the outlay runs out,
// and quietly superseded. A directory that tells a street vendor to apply to
// a window that closed last month is worse than no directory. So before any
// scheme is shown, its name goes through Google News and every card carries a
// verified-at stamp.
package freshness

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// closedSignals suggest the scheme is no longer running.
var closedSignals = []string{
	"discontinued", "scrapped", "withdrawn", "suspended", "halted",
	"no longer", "closed", "wound up", "subsumed", "replaced by",
	"stopped", "shelved",
}

// changedSignals suggest the rules or dates moved but the scheme is alive.
var changedSignals = []string{
	"extended", "deadline", "last date", "revised", "hiked", "raised",
	"increased", "reduced", "new guidelines", "amended", "relaunched",
	"reopened", "enhanced", "expanded", "cap raised", "limit raised",
}

// Checker verifies scheme status against live news coverage.
type Checker struct {
	client *serp.Client
	// Concurrency limits parallel SerpApi calls.
	Concurrency int
	// MaxSchemes caps how many schemes get a freshness call per request.
	// Credits are finite, so only the top-ranked matches are verified.
	MaxSchemes int
}

// New builds a Checker.
func New(client *serp.Client) *Checker {
	return &Checker{client: client, Concurrency: 4, MaxSchemes: 5}
}

// Check runs the news verification for a single scheme.
func (c *Checker) Check(ctx context.Context, s models.Scheme) (*models.Freshness, error) {
	name := s.Name
	if len(s.SearchKeywords) > 0 {
		name = s.SearchKeywords[0]
	}

	params := serp.Params{
		"engine": serp.EngineGoogleNews,
		"q":      fmt.Sprintf("%q deadline OR extended OR closed OR revised OR suspended", name),
		"gl":     "in",
		"hl":     "en",
	}

	resp, err := c.client.Search(ctx, params)
	if err != nil {
		if errors.Is(err, serp.ErrCacheMiss) || errors.Is(err, serp.ErrCreditBudget) {
			return &models.Freshness{
				Status:    models.FreshnessUnknown,
				Note:      "Not verified on this run (no live search budget). Confirm on the official portal before applying.",
				CheckedAt: time.Now(),
			}, nil
		}
		return nil, err
	}

	return classify(resp.News, resp.FetchedAt), nil
}

// CheckAll verifies the top matches in parallel and attaches the results in
// place.
func (c *Checker) CheckAll(ctx context.Context, matches []models.Match) []string {
	limit := c.MaxSchemes
	if limit <= 0 || limit > len(matches) {
		limit = len(matches)
	}

	var (
		mu       sync.Mutex
		warnings []string
	)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(max(1, c.Concurrency))

	for i := range matches[:limit] {
		g.Go(func() error {
			f, err := c.Check(ctx, matches[i].Scheme)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, fmt.Sprintf("freshness check failed for %s: %v", matches[i].Scheme.ID, err))
				mu.Unlock()
				return nil
			}
			matches[i].Freshness = f
			return nil
		})
	}
	_ = g.Wait()

	// Schemes past the verification cap are still labelled, so the UI never
	// implies something was checked when it wasn't.
	for i := limit; i < len(matches); i++ {
		matches[i].Freshness = &models.Freshness{
			Status:    models.FreshnessUnknown,
			Note:      "Below the per-request verification limit. Re-run with this scheme in the top results to verify it.",
			CheckedAt: time.Now(),
		}
	}

	return warnings
}

func classify(news []serp.NewsResult, fetchedAt time.Time) *models.Freshness {
	checkedAt := fetchedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}

	if len(news) == 0 {
		return &models.Freshness{
			Status:    models.FreshnessUnknown,
			Note:      "No recent news coverage found. Absence of news usually means no change, but confirm the last date on the official portal.",
			CheckedAt: checkedAt,
		}
	}

	var (
		status   = models.FreshnessOK
		note     string
		evidence []models.Evidence
	)

	for _, n := range news {
		text := strings.ToLower(n.Title + " " + n.Snippet)

		if hit := firstMatch(text, closedSignals); hit != "" {
			// Closed beats changed: it is the finding that must not be
			// missed, so it wins regardless of what else matched.
			status = models.FreshnessClosed
			note = fmt.Sprintf("Coverage mentions %q. Verify on the official portal before applying.", hit)
			evidence = append([]models.Evidence{toEvidence(n)}, evidence...)
			continue
		}

		if hit := firstMatch(text, changedSignals); hit != "" {
			if status != models.FreshnessClosed {
				status = models.FreshnessChanged
				if note == "" {
					note = fmt.Sprintf("Coverage mentions %q - dates or limits may have moved since the catalog was written.", hit)
				}
			}
			evidence = append(evidence, toEvidence(n))
		}
	}

	if status == models.FreshnessOK {
		note = "No news indicating a change, closure or deadline shift."
		// Keep a couple of citations so the stamp is checkable rather than
		// just asserted.
		for i, n := range news {
			if i >= 2 {
				break
			}
			evidence = append(evidence, toEvidence(n))
		}
	}

	if len(evidence) > 4 {
		evidence = evidence[:4]
	}

	return &models.Freshness{
		Status:    status,
		Note:      note,
		Evidence:  evidence,
		CheckedAt: checkedAt,
	}
}

func toEvidence(n serp.NewsResult) models.Evidence {
	return models.Evidence{
		Title:  strings.TrimSpace(n.Title),
		Link:   n.Link,
		Source: n.SourceName(),
		Date:   n.Date,
	}
}

func firstMatch(text string, signals []string) string {
	for _, s := range signals {
		if strings.Contains(text, s) {
			return s
		}
	}
	return ""
}
