package discovery

import (
	"slices"
	"strings"
	"testing"

	"github.com/manoj-2003/scheme-setu/internal/models"
)

func TestDomain(t *testing.T) {
	cases := map[string]string{
		"https://www.pmkisan.gov.in/":                  "pmkisan.gov.in",
		"https://msmeonline.tn.gov.in/scheme?id=4":     "msmeonline.tn.gov.in",
		"http://NHFDC.NIC.IN/loan.pdf":                 "nhfdc.nic.in",
		"not a url":                                    "",
		"https://tn.gov.in/scheme/guidelines_2026.pdf": "tn.gov.in",
	}
	for in, want := range cases {
		if got := Domain(in); got != want {
			t.Errorf("Domain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsGovDomain(t *testing.T) {
	for _, host := range []string{"gov.in", "nic.in", "tn.gov.in", "pmkisan.gov.in", "nhfdc.nic.in"} {
		if !IsGovDomain(host) {
			t.Errorf("%s should be a government domain", host)
		}
	}
	for _, host := range []string{"pmkisan-gov.in", "gov.in.apply.com", "sarkariyojana.com", ""} {
		if IsGovDomain(host) {
			t.Errorf("%s should NOT be a government domain", host)
		}
	}
}

// The credit budget is the binding constraint on this project, so the query
// planner must never exceed the cap it is given.
func TestPlanRespectsQueryCap(t *testing.T) {
	p := models.Profile{
		State:      "tamil_nadu",
		Occupation: models.OccupationStreetVendor,
		Purpose:    models.PurposeWorkingCapital,
		Category:   models.CategoryOBC,
	}

	for _, cap := range []int{1, 3, 6} {
		if got := len(Plan(p, []string{"en", "hi", "ta"}, cap)); got > cap {
			t.Errorf("Plan with cap %d produced %d queries", cap, got)
		}
	}
}

// Site restriction is what reaches state circulars. Losing it silently turns
// discovery into a generic web search full of aggregator blogspam.
func TestPlanSiteRestrictsToStatePortal(t *testing.T) {
	p := models.Profile{
		State:      "tamil_nadu",
		Occupation: models.OccupationStreetVendor,
		Purpose:    models.PurposeWorkingCapital,
	}

	plan := Plan(p, []string{"en"}, 6)
	var found bool
	for _, q := range plan {
		if q.Label == "state_portal" && strings.Contains(q.Params.Query(), "site:tn.gov.in") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no site-restricted Tamil Nadu portal query in plan: %+v", plan)
	}
}

func TestPlanUsesApplicantLanguage(t *testing.T) {
	p := models.Profile{
		State:      "west_bengal",
		Occupation: models.OccupationStudent,
		Purpose:    models.PurposeEducation,
		Language:   "bn",
	}

	plan := Plan(p, []string{"bn", "en"}, 6)
	for _, q := range plan {
		if q.Language == "bn" {
			return
		}
	}
	t.Fatal("plan contains no Bengali query for a Bengali-speaking applicant")
}

// The configured list is a default for profiles that name no language, not a
// set to be unioned with the applicant's choice. Unioning it meant a Tamil
// applicant also searched Hindi, and since Plan truncates to MaxQueries those
// wasted slots pushed the guidelines-PDF query out of the plan entirely.
func TestLanguagesForReplacesConfiguredList(t *testing.T) {
	configured := []string{"en", "hi"}

	tamil := LanguagesFor(models.Profile{Language: "ta"}, configured)
	if want := []string{"ta", "en"}; !slices.Equal(tamil, want) {
		t.Errorf("Tamil applicant: got %v, want %v", tamil, want)
	}

	// No language named: the configured breadth still applies, which is what
	// cmd/warm and the demo personas rely on.
	none := LanguagesFor(models.Profile{}, configured)
	if !slices.Equal(none, configured) {
		t.Errorf("unspecified language: got %v, want %v", none, configured)
	}
}

// Regression: a non-English applicant must not lose the filetype:pdf query,
// which is how state schemes published only as a circular are reached.
func TestPlanKeepsGuidelinesPDFForNonEnglishApplicant(t *testing.T) {
	p := models.Profile{
		State:      "tamil_nadu",
		Category:   models.CategoryOBC,
		Occupation: models.OccupationStreetVendor,
		Purpose:    models.PurposeWorkingCapital,
		Language:   "ta",
	}

	plan := Plan(p, LanguagesFor(p, []string{"en", "hi"}), 6)

	var labels []string
	for _, q := range plan {
		labels = append(labels, q.Label)
		if q.Language == "hi" {
			t.Errorf("Tamil applicant got a Hindi query: %s", q.Params.Query())
		}
	}
	if !slices.Contains(labels, "guidelines_pdf") {
		t.Errorf("guidelines_pdf truncated out of the plan; labels were %v", labels)
	}
}

func TestLooksLikeScheme(t *testing.T) {
	yes := []string{
		"pm svanidhi scheme for street vendors",
		"मुख्यमंत्री स्वरोजगार योजना",
		"capital subsidy guidelines for msme",
		"தமிழ்நாடு திட்டம் விவரம்",
	}
	for _, s := range yes {
		if !looksLikeScheme(strings.ToLower(s)) {
			t.Errorf("%q should look like a scheme page", s)
		}
	}

	if looksLikeScheme("district collector office contact directory") {
		t.Error("a contact directory should not look like a scheme page")
	}
}
