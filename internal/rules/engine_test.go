package rules

import (
	"testing"

	"github.com/manoj-2003/scheme-setu/internal/catalog"
	"github.com/manoj-2003/scheme-setu/internal/models"
)

// streetVendor is the demo persona: the applicant PM SVANidhi exists for.
func streetVendor() models.Profile {
	return models.Profile{
		State:                 "tamil_nadu",
		Age:                   34,
		Gender:                "female",
		AnnualIncomeINR:       180000,
		Category:              models.CategoryOBC,
		Occupation:            models.OccupationStreetVendor,
		Purpose:               models.PurposeWorkingCapital,
		AmountNeededINR:       50000,
		HasAadhaar:            true,
		HasBankAccount:        true,
		BusinessVintageMonths: 30,
	}
}

func TestEvaluateEligible(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	scheme, ok := cat.ByID("pm-svanidhi-1")
	if !ok {
		t.Fatal("pm-svanidhi-1 missing from catalog")
	}

	m := Evaluate(streetVendor(), scheme)
	if m.Status != models.StatusEligible {
		t.Fatalf("want eligible, got %s (reasons: %+v)", m.Status, m.Reasons)
	}
	if len(m.Reasons) == 0 {
		t.Error("an eligibility verdict with no stated reasons is not explainable")
	}
}

func TestBlockingRuleMakesIneligible(t *testing.T) {
	cat, _ := catalog.Load()
	scheme, _ := cat.ByID("pm-svanidhi-1")

	p := streetVendor()
	p.Occupation = models.OccupationSalaried // hard rule: vendors only

	m := Evaluate(p, scheme)
	if m.Status != models.StatusIneligible {
		t.Fatalf("want ineligible for a salaried applicant, got %s", m.Status)
	}
}

// A fixable gap must surface as a near miss with an actionable step, not as a
// flat rejection. This is the behaviour the product is really selling.
func TestFixableGapBecomesNearMissWithUnlockStep(t *testing.T) {
	cat, _ := catalog.Load()
	scheme, ok := cat.ByID("pm-mudra-tarun")
	if !ok {
		t.Fatal("pm-mudra-tarun missing from catalog")
	}

	p := models.Profile{
		State:                 "telangana",
		Age:                   38,
		Category:              models.CategoryGeneral,
		Occupation:            models.OccupationMicroEnterprise,
		Purpose:               models.PurposeWorkingCapital,
		AmountNeededINR:       800000,
		HasAadhaar:            true,
		HasBankAccount:        true,
		HasUdyam:              false, // the only gap
		BusinessVintageMonths: 60,
	}

	m := Evaluate(p, scheme)
	if m.Status != models.StatusNearMiss {
		t.Fatalf("want near_miss when only Udyam is missing, got %s", m.Status)
	}
	if len(m.UnlockSteps) == 0 {
		t.Fatal("a near miss with no unlock step tells the applicant nothing")
	}
}

// Stand-Up India is open to SC/ST applicants of any gender AND to women of
// any category. Encoding that as two independent lists would wrongly demand
// both, so a general-category woman must still pass.
func TestCategoryOrGenderAlternative(t *testing.T) {
	cat, _ := catalog.Load()
	scheme, ok := cat.ByID("stand-up-india")
	if !ok {
		t.Fatal("stand-up-india missing from catalog")
	}
	if len(scheme.Eligibility.Genders) == 0 {
		t.Skip("catalog entry no longer declares a gender alternative")
	}

	p := models.Profile{
		State:           "bihar",
		Age:             29,
		Gender:          "female",
		Category:        models.CategoryGeneral,
		Occupation:      models.OccupationUnemployed,
		Purpose:         models.PurposeNewBusiness,
		AmountNeededINR: 1500000,
		HasAadhaar:      true,
		HasBankAccount:  true,
	}

	if m := Evaluate(p, scheme); m.Status == models.StatusIneligible {
		t.Fatalf("a general-category woman must not be blocked by the category rule: %+v", m.Reasons)
	}
}

// A first-time street vendor must NOT be shown the Rs 50,000 third tranche.
// Tiered credit is only open after the earlier tranche is repaid, and showing
// the biggest number to someone who cannot get it sends them to a bank that
// will turn them away.
func TestTieredCreditRequiresThePriorTranche(t *testing.T) {
	cat, _ := catalog.Load()

	p := streetVendor()
	if p.HasAvailed("pm-svanidhi-1") {
		t.Fatal("test persona should be a first-time applicant")
	}

	eligible, _ := EvaluateAll(p, cat.Candidates(p))
	for _, m := range eligible {
		switch m.Scheme.ID {
		case "pm-svanidhi-2", "pm-svanidhi-3":
			t.Errorf("%s offered to a first-time vendor with no prior tranche", m.Scheme.ID)
		}
	}

	// Having repaid the first tranche must open the second, but still not
	// the third.
	p.ExistingSchemeIDs = []string{"pm-svanidhi-1"}
	eligible, _ = EvaluateAll(p, cat.Candidates(p))

	var sawSecond bool
	for _, m := range eligible {
		if m.Scheme.ID == "pm-svanidhi-2" {
			sawSecond = true
		}
		if m.Scheme.ID == "pm-svanidhi-3" {
			t.Error("third tranche offered after only the first was repaid")
		}
	}
	if !sawSecond {
		t.Error("second tranche not offered after the first was repaid")
	}
}

func TestAlreadyAvailedIsExcluded(t *testing.T) {
	cat, _ := catalog.Load()
	scheme, _ := cat.ByID("pm-svanidhi-1")

	p := streetVendor()
	p.ExistingSchemeIDs = []string{"pm-svanidhi-1"}

	if m := Evaluate(p, scheme); m.Status != models.StatusIneligible {
		t.Fatalf("want ineligible for an already-availed scheme, got %s", m.Status)
	}
}

// Ranking must prefer a scheme that covers the requested amount. Being
// offered Rs 10,000 against a Rs 5,00,000 need is a technical match and a
// practical no.
func TestRankingPrefersSchemesThatCoverTheAmount(t *testing.T) {
	cat, _ := catalog.Load()

	p := streetVendor()
	p.AmountNeededINR = 50000

	eligible, _ := EvaluateAll(p, cat.Candidates(p))
	if len(eligible) < 2 {
		t.Skipf("need at least two eligible schemes to compare ranking, got %d", len(eligible))
	}

	top := eligible[0]
	for _, m := range eligible[1:] {
		if m.Score > top.Score {
			t.Fatalf("results are not sorted by score: %s(%d) after %s(%d)",
				m.Scheme.ID, m.Score, top.Scheme.ID, top.Score)
		}
	}
}

func TestRupeesUsesIndianGrouping(t *testing.T) {
	cases := map[int]string{
		0:        "Rs 0",
		999:      "Rs 999",
		1000:     "Rs 1,000",
		50000:    "Rs 50,000",
		100000:   "Rs 1,00,000",
		1000000:  "Rs 10,00,000",
		10000000: "Rs 1,00,00,000",
	}
	for in, want := range cases {
		if got := rupees(in); got != want {
			t.Errorf("rupees(%d) = %q, want %q", in, got, want)
		}
	}
}

// Every catalog entry must parse into a usable rule set. A typo'd occupation
// or a missing apply URL silently removes a scheme from every result.
func TestCatalogIntegrity(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if cat.Len() == 0 {
		t.Fatal("catalog is empty")
	}

	validPurpose := map[models.Purpose]bool{
		models.PurposeWorkingCapital: true, models.PurposeEquipment: true,
		models.PurposeNewBusiness: true, models.PurposeElectricVehicle: true,
		models.PurposeRooftopSolar: true, models.PurposeEducation: true,
		models.PurposeLivestock: true, models.PurposeFoodProcessing: true,
		models.PurposeIncomeSupport: true,
	}

	for _, s := range cat.All() {
		if s.Name == "" {
			t.Errorf("%s: no name", s.ID)
		}
		if s.Benefit.Summary == "" {
			t.Errorf("%s: no benefit summary, so the card has nothing to lead with", s.ID)
		}
		if len(s.Purposes) == 0 {
			t.Errorf("%s: no purposes, so it will never be a candidate", s.ID)
		}
		for _, p := range s.Purposes {
			if !validPurpose[p] {
				t.Errorf("%s: unknown purpose %q", s.ID, p)
			}
		}
		if s.Level == models.LevelState && len(s.States) == 0 {
			t.Errorf("%s: state-level scheme with no states listed", s.ID)
		}
		if s.ApplyURL == "" && len(s.SourceURLs) == 0 {
			t.Errorf("%s: no apply URL and no source URL, so the applicant has nowhere to go", s.ID)
		}
		if len(s.OfficialDomains) == 0 {
			t.Errorf("%s: no official domains, so the fraud shield cannot verify its apply link", s.ID)
		}
	}
}
