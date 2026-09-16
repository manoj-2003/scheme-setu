// Package fraud is the shield against fee-charging lookalike portals.
//
// Search any Indian scheme name and page one carries official portals mixed
// with sites that copy the government layout, rank on the scheme's name, and
// charge a "processing fee" to fill a free form. The people worst served by
// scheme information are the ones least able to tell the difference. So this
// package does two things: it confirms the apply link we hand out resolves to
// a genuinely official domain, and it actively searches for the impostors so
// the applicant can be warned about them by name before they land there.
package fraud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/manoj-2003/scheme-setu/internal/discovery"
	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// Additional domains that are legitimate but not under gov.in/nic.in --
// public-sector banks, statutory corporations and scheme portals on their own
// TLD. Anything outside this set and the gov.in/nic.in space is treated as
// unofficial.
var extraOfficialDomains = []string{
	"mudra.org.in",
	"sidbi.in",
	"nabard.org",
	"cgtmse.in",
	"standupmitra.in",
	"udyamimitra.in",
	"jansamarth.in",
	"vidyalakshmi.co.in",
	"onlinesbi.sbi",
	"sbi.co.in",
	"bankofbaroda.in",
	"pnbindia.in",
	"unionbankofindia.co.in",
	"canarabank.com",
	"indianbank.in",
	"bankofindia.co.in",
	"rbi.org.in",
	"npci.org.in",
}

// solicitationSignals mark a page that is trying to take an application,
// which is what makes an unofficial domain dangerous rather than merely
// irrelevant. A news article about a scheme is fine; a form is not.
var solicitationSignals = []string{
	"apply online", "apply now", "registration", "register now",
	"application form", "online form", "fill the form", "apply here",
	"processing fee", "service charge", "pay now", "helpline",
	"आवेदन करें", "ऑनलाइन आवेदन", "पंजीकरण",
}

// Checker verifies apply links and hunts for impostors.
type Checker struct {
	client *serp.Client
	allow  map[string]bool
}

// New builds a Checker trusting the catalog's declared domains on top of the
// built-in allowlist.
func New(client *serp.Client, catalogDomains []string) *Checker {
	allow := make(map[string]bool, len(catalogDomains)+len(extraOfficialDomains))
	for _, d := range extraOfficialDomains {
		allow[strings.ToLower(d)] = true
	}
	for _, d := range catalogDomains {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			allow[d] = true
		}
	}
	return &Checker{client: client, allow: allow}
}

// IsOfficial reports whether a host is trustworthy: an Indian government
// domain, or an explicitly allowlisted institution.
func (c *Checker) IsOfficial(host string) bool {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	if host == "" {
		return false
	}
	if discovery.IsGovDomain(host) {
		return true
	}
	if c.allow[host] {
		return true
	}
	// Allow subdomains of allowlisted institutions (retail.onlinesbi.sbi and
	// the like) without allowing lookalikes that merely embed the name.
	for d := range c.allow {
		if strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Check verifies one scheme's apply link and searches for lookalikes.
func (c *Checker) Check(ctx context.Context, s models.Scheme) (*models.TrustReport, error) {
	report := &models.TrustReport{CheckedAt: time.Now()}

	if s.ApplyURL != "" {
		report.ApplyDomain = discovery.Domain(s.ApplyURL)
		report.ApplyURLOfficial = c.IsOfficial(report.ApplyDomain)
	}

	name := s.Name
	if len(s.SearchKeywords) > 0 {
		name = s.SearchKeywords[0]
	}

	// Deliberately the query a worried applicant would type. The impostors
	// optimise for exactly this phrasing, which is why it finds them.
	params := serp.Params{
		"engine":        serp.EngineGoogle,
		"q":             fmt.Sprintf("%q apply online registration", name),
		"google_domain": "google.co.in",
		"gl":            "in",
		"hl":            "en",
		"num":           "10",
	}

	resp, err := c.client.Search(ctx, params)
	if err != nil {
		if errors.Is(err, serp.ErrCacheMiss) || errors.Is(err, serp.ErrCreditBudget) {
			// No budget to hunt impostors on this run. The apply-link check
			// above is offline and still stands.
			return report, nil
		}
		return nil, err
	}

	for _, r := range resp.Organic {
		host := discovery.Domain(r.Link)
		if host == "" || c.IsOfficial(host) {
			continue
		}

		text := strings.ToLower(r.Title + " " + r.Snippet)
		hit := firstMatch(text, solicitationSignals)
		if hit == "" {
			// Ranks for the scheme name but isn't soliciting applications --
			// a blog or news piece. Not a fraud signal.
			continue
		}

		report.SuspectedImpostors = append(report.SuspectedImpostors, models.Impostor{
			Domain: host,
			Title:  strings.TrimSpace(r.Title),
			Link:   r.Link,
			Why: fmt.Sprintf(
				"Ranks at position %d for this scheme and invites you to %q, but %s is not a government (gov.in / nic.in) or recognised institutional domain.",
				r.Position, hit, host),
		})
	}

	if len(report.SuspectedImpostors) > 5 {
		report.SuspectedImpostors = report.SuspectedImpostors[:5]
	}
	return report, nil
}

func firstMatch(text string, signals []string) string {
	for _, s := range signals {
		if strings.Contains(text, s) {
			return s
		}
	}
	return ""
}
