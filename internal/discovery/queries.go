// Package discovery finds schemes that are not in the curated catalog.
//
// Central schemes are well indexed and easy to hand-curate. State schemes are
// not: they live on 30-odd state portals, often only in the local language,
// frequently as a PDF circular with no HTML page. That long tail is where
// search earns its place in this project, and it is why the same queries are
// re-run with hl=hi/ta/bn rather than just translating the UI.
package discovery

import (
	"fmt"
	"strings"
	"time"

	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// Query is one intended SerpApi call, labelled so the response can explain
// which search produced which lead.
type Query struct {
	Label    string      `json:"label"`
	Language string      `json:"language"`
	Params   serp.Params `json:"params"`
	Weight   float64     `json:"weight"`
}

// stateDomains maps a state slug to its official web domain, so discovery can
// use site: restriction instead of hoping the state name appears in the page.
var stateDomains = map[string]string{
	"andhra_pradesh":    "ap.gov.in",
	"arunachal_pradesh": "arunachalpradesh.gov.in",
	"assam":             "assam.gov.in",
	"bihar":             "bihar.gov.in",
	"chhattisgarh":      "cgstate.gov.in",
	"delhi":             "delhi.gov.in",
	"goa":               "goa.gov.in",
	"gujarat":           "gujarat.gov.in",
	"haryana":           "haryana.gov.in",
	"himachal_pradesh":  "himachal.nic.in",
	"jammu_and_kashmir": "jk.gov.in",
	"jharkhand":         "jharkhand.gov.in",
	"karnataka":         "karnataka.gov.in",
	"kerala":            "kerala.gov.in",
	"madhya_pradesh":    "mp.gov.in",
	"maharashtra":       "maharashtra.gov.in",
	"manipur":           "manipur.gov.in",
	"meghalaya":         "meghalaya.gov.in",
	"mizoram":           "mizoram.gov.in",
	"nagaland":          "nagaland.gov.in",
	"odisha":            "odisha.gov.in",
	"punjab":            "punjab.gov.in",
	"rajasthan":         "rajasthan.gov.in",
	"sikkim":            "sikkim.gov.in",
	"tamil_nadu":        "tn.gov.in",
	"telangana":         "telangana.gov.in",
	"tripura":           "tripura.gov.in",
	"uttar_pradesh":     "up.gov.in",
	"uttarakhand":       "uk.gov.in",
	"west_bengal":       "wb.gov.in",
}

// purposeTerms are the words the portals themselves use. "working_capital"
// returns nothing; "working capital loan" returns the circular.
var purposeTerms = map[models.Purpose]string{
	models.PurposeWorkingCapital:  "working capital loan",
	models.PurposeEquipment:       "machinery equipment subsidy loan",
	models.PurposeNewBusiness:     "self employment new enterprise loan subsidy",
	models.PurposeElectricVehicle: "electric vehicle subsidy incentive",
	models.PurposeRooftopSolar:    "rooftop solar subsidy",
	models.PurposeEducation:       "education loan scholarship interest subsidy",
	models.PurposeLivestock:       "dairy poultry livestock subsidy loan",
	models.PurposeFoodProcessing:  "food processing unit subsidy",
	models.PurposeIncomeSupport:   "income support financial assistance",
}

var occupationTerms = map[models.Occupation]string{
	models.OccupationFarmer:          "farmer",
	models.OccupationStreetVendor:    "street vendor",
	models.OccupationArtisan:         "artisan handicraft weaver",
	models.OccupationMicroEnterprise: "MSME micro enterprise",
	models.OccupationSelfEmployed:    "self employed entrepreneur",
	models.OccupationSalaried:        "salaried",
	models.OccupationStudent:         "student",
	models.OccupationUnemployed:      "unemployed youth",
}

var categoryTerms = map[models.Category]string{
	models.CategorySC:       "scheduled caste SC",
	models.CategoryST:       "scheduled tribe ST",
	models.CategoryOBC:      "OBC backward class",
	models.CategoryMinority: "minority",
	models.CategoryEWS:      "economically weaker section",
}

// Plan builds the query set for a profile. maxQueries is a hard cap because
// the free SerpApi plan grants 250 credits a month; a single wizard
// submission must not cost more than a handful.
func Plan(p models.Profile, langs []string, maxQueries int) []Query {
	if len(langs) == 0 {
		langs = []string{"en"}
	}
	if maxQueries <= 0 {
		maxQueries = 6
	}

	year := time.Now().Year()
	state := stateSlug(p.State)
	stateName := displayState(p.State)
	purpose := purposeTerms[p.Purpose]
	occupation := occupationTerms[p.Occupation]
	domain := stateDomains[state]

	base := func(q, lang string) serp.Params {
		return serp.Params{
			"engine":        serp.EngineGoogle,
			"q":             q,
			"google_domain": "google.co.in",
			"gl":            "in",
			"hl":            lang,
			"num":           "10",
		}
	}

	var queries []Query

	// 1. State portal, site-restricted. The highest-value query in the whole
	//    project: this is what surfaces schemes no aggregator lists.
	if domain != "" {
		for _, lang := range langs {
			queries = append(queries, Query{
				Label:    "state_portal",
				Language: lang,
				Weight:   1.0,
				Params:   base(fmt.Sprintf("site:%s %s %s scheme apply online", domain, purpose, occupation), lang),
			})
		}
	}

	// 2. Any government domain, state named. Catches district sites, mission
	//    directorates and corporation portals outside the main state domain.
	for _, lang := range langs {
		queries = append(queries, Query{
			Label:    "gov_wide_state",
			Language: lang,
			Weight:   0.85,
			Params:   base(fmt.Sprintf("site:*.gov.in %s %s scheme %s %d", stateName, purpose, occupation, year), lang),
		})
	}

	// 3. Category-specific corporations (SC/ST/OBC/minority finance
	//    corporations) run their own loan schemes that no general search
	//    for the purpose will return.
	if term, ok := categoryTerms[p.Category]; ok {
		queries = append(queries, Query{
			Label:    "category_corporation",
			Language: langs[0],
			Weight:   0.8,
			Params:   base(fmt.Sprintf("site:*.gov.in %s %s finance corporation loan scheme %s", stateName, term, purpose), langs[0]),
		})
	}

	// 4. Guidelines PDFs. State schemes are very often published only as a
	//    circular, and filetype:pdf is how you reach them.
	queries = append(queries, Query{
		Label:    "guidelines_pdf",
		Language: langs[0],
		Weight:   0.7,
		Params:   base(fmt.Sprintf("site:*.gov.in %s %s scheme guidelines filetype:pdf", stateName, purpose), langs[0]),
	})

	// 5. Autocomplete. Not a scheme source -- it tells us what applicants
	//    actually type, which we surface as related questions and use to seed
	//    better queries on the next run.
	queries = append(queries, Query{
		Label:    "autocomplete",
		Language: langs[0],
		Weight:   0.3,
		Params: serp.Params{
			"engine": serp.EngineGoogleAutocomplete,
			"q":      fmt.Sprintf("%s scheme for %s in %s", purpose, occupation, stateName),
			"gl":     "in",
			"hl":     langs[0],
		},
	})

	if len(queries) > maxQueries {
		queries = queries[:maxQueries]
	}
	return queries
}

func stateSlug(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
}

// displayState turns a slug back into the words that appear on the portals.
func displayState(s string) string {
	parts := strings.Split(stateSlug(s), "_")
	for i, p := range parts {
		if p == "and" {
			continue
		}
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// StateDomain exposes the portal domain for a state, for the fraud shield's
// allowlist.
func StateDomain(state string) string { return stateDomains[stateSlug(state)] }

// States returns every supported state slug, for the wizard dropdown.
func States() []string {
	out := make([]string, 0, len(stateDomains))
	for s := range stateDomains {
		out = append(out, s)
	}
	return out
}
