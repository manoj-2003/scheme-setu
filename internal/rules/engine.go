// Package rules evaluates a profile against a scheme's structured
// eligibility and explains itself.
//
// The explanation is the point. A bare yes/no is not trustworthy when the
// subject is a loan, and the near-miss output ("you would qualify if you
// registered on Udyam") is often more useful to a first-time applicant than
// the matches themselves.
package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/manoj-2003/scheme-setu/internal/models"
)

// Evaluate runs every rule for one scheme against one profile.
func Evaluate(p models.Profile, s models.Scheme) models.Match {
	reasons := make([]models.Reason, 0, 12)
	var unlock []string

	add := func(rule string, passed, blocking bool, format string, args ...any) {
		reasons = append(reasons, models.Reason{
			Rule:     rule,
			Passed:   passed,
			Blocking: blocking,
			Detail:   fmt.Sprintf(format, args...),
		})
	}

	e := s.Eligibility

	// --- Hard rules: nothing the applicant can do about these today. ---

	if e.MinAge > 0 || e.MaxAge > 0 {
		ok := (e.MinAge == 0 || p.Age >= e.MinAge) && (e.MaxAge == 0 || p.Age <= e.MaxAge)
		add("age", ok, true, "%s (you are %d)", ageRangeText(e.MinAge, e.MaxAge), p.Age)
	}

	if e.MaxAnnualIncomeINR > 0 {
		ok := p.AnnualIncomeINR <= e.MaxAnnualIncomeINR
		add("income", ok, true, "Family income must not exceed %s (yours: %s)",
			rupees(e.MaxAnnualIncomeINR), rupees(p.AnnualIncomeINR))
	}

	if len(e.Categories) > 0 {
		// Several schemes open to SC/ST are also open to women of any
		// category. Encoding that as a category list plus a gender list would
		// wrongly require both, so treat the two as alternatives.
		ok := containsCategory(e.Categories, p.Category) ||
			(len(e.Genders) > 0 && containsFold(e.Genders, p.Gender))
		add("category", ok, true, "Reserved for %s%s (you selected %s)",
			joinCategories(e.Categories),
			genderAlternativeText(e.Genders),
			strings.ToUpper(string(p.Category)))
	} else if len(e.Genders) > 0 {
		ok := containsFold(e.Genders, p.Gender)
		add("gender", ok, true, "Open to %s applicants", strings.Join(e.Genders, ", "))
	}

	if len(e.Occupations) > 0 {
		ok := containsOccupation(e.Occupations, p.Occupation)
		add("occupation", ok, true, "For %s (you selected %s)",
			joinOccupations(e.Occupations), humanize(string(p.Occupation)))
	}

	if e.MaxLandHoldingAcres != nil {
		ok := p.LandHoldingAcres <= *e.MaxLandHoldingAcres
		add("land_holding", ok, true, "Land holding must not exceed %.2f acres (yours: %.2f)",
			*e.MaxLandHoldingAcres, p.LandHoldingAcres)
	}

	if e.MaxAnnualTurnoverINR > 0 {
		ok := p.AnnualTurnoverINR <= e.MaxAnnualTurnoverINR
		add("turnover", ok, true, "Annual turnover must not exceed %s (yours: %s)",
			rupees(e.MaxAnnualTurnoverINR), rupees(p.AnnualTurnoverINR))
	}

	if e.RequiresDisability {
		add("disability", p.HasDisability, true,
			"Requires a benchmark disability certificate (40%% or more)")
	}

	for _, excluded := range e.ExcludesSchemeIDs {
		if p.HasAvailed(excluded) {
			add("mutual_exclusion", false, true,
				"Not available to existing beneficiaries of %s", excluded)
		}
	}

	// Tiered credit: the higher tranche is only open to someone who has
	// already repaid the lower one. Treated as blocking, because the gap
	// cannot be closed by paperwork -- it takes a completed loan cycle.
	for _, required := range e.RequiresSchemeIDs {
		ok := p.HasAvailed(required)
		add("prerequisite", ok, true,
			"Requires an earlier %s loan, fully repaid on schedule", prettySchemeID(required))
	}

	if p.HasAvailed(s.ID) {
		add("already_availed", false, true, "You have marked this scheme as already availed")
	}

	// --- Soft rules: fixable, and each becomes an unlock step. ---

	if e.RequiresAadhaar {
		ok := p.HasAadhaar
		add("aadhaar", ok, false, "Aadhaar is mandatory for this scheme")
		if !ok {
			unlock = append(unlock, "Get an Aadhaar number, or link your existing Aadhaar to your mobile number, at the nearest Aadhaar Seva Kendra.")
		}
	}

	if e.RequiresBankAccount {
		ok := p.HasBankAccount
		add("bank_account", ok, false, "Disbursal is direct-to-account, so a bank account is required")
		if !ok {
			unlock = append(unlock, "Open a zero-balance Jan Dhan account at any bank branch or post office. Aadhaar is enough to open it.")
		}
	}

	if e.RequiresUdyam {
		ok := p.HasUdyam
		add("udyam", ok, false, "Requires Udyam (MSME) registration")
		if !ok {
			unlock = append(unlock, "Register free on the Udyam portal (udyamregistration.gov.in). It takes minutes with Aadhaar and PAN, and unlocks several MSME schemes at once.")
		}
	}

	if e.MinBusinessVintageMonths > 0 {
		ok := p.BusinessVintageMonths >= e.MinBusinessVintageMonths
		add("business_vintage", ok, false, "Business should be at least %d months old (yours: %d)",
			e.MinBusinessVintageMonths, p.BusinessVintageMonths)
		if !ok {
			shortfall := e.MinBusinessVintageMonths - p.BusinessVintageMonths
			unlock = append(unlock, fmt.Sprintf(
				"Your business needs about %d more month(s) of operating history. In the meantime a smaller MUDRA Shishu loan has no vintage requirement.", shortfall))
		}
	}

	status := verdict(reasons)
	return models.Match{
		Scheme:      s,
		Status:      status,
		Score:       score(p, s, status),
		Reasons:     reasons,
		UnlockSteps: unlock,
		Origin:      "catalog",
	}
}

// EvaluateAll runs the engine over a set of schemes and returns the eligible
// and near-miss buckets, each sorted best-first. Ineligible schemes are
// dropped: telling a 24-year-old they failed an age rule is noise.
func EvaluateAll(p models.Profile, schemes []models.Scheme) (eligible, nearMiss []models.Match) {
	for _, s := range schemes {
		m := Evaluate(p, s)
		switch m.Status {
		case models.StatusEligible:
			eligible = append(eligible, m)
		case models.StatusNearMiss:
			nearMiss = append(nearMiss, m)
		}
	}

	sortMatches(eligible)
	sortMatches(nearMiss)
	return eligible, nearMiss
}

func sortMatches(ms []models.Match) {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].Score != ms[j].Score {
			return ms[i].Score > ms[j].Score
		}
		return ms[i].Scheme.Name < ms[j].Scheme.Name
	})
}

func verdict(reasons []models.Reason) models.MatchStatus {
	softFail := false
	for _, r := range reasons {
		if r.Passed {
			continue
		}
		if r.Blocking {
			return models.StatusIneligible
		}
		softFail = true
	}
	if softFail {
		return models.StatusNearMiss
	}
	return models.StatusEligible
}

// score ranks matches by how well the scheme fits what the applicant asked
// for. Amount fit dominates: being offered Rs 10,000 when you need Rs 5 lakh
// is technically a match and practically useless.
func score(p models.Profile, s models.Scheme, status models.MatchStatus) int {
	total := 50

	if status == models.StatusNearMiss {
		total -= 20
	}

	if p.AmountNeededINR > 0 {
		switch {
		case s.Benefit.MaxAmountINR == 0:
			// Subsidy-style benefit with no stated ceiling; neutral.
		case p.AmountNeededINR <= s.Benefit.MaxAmountINR && p.AmountNeededINR >= s.Benefit.MinAmountINR:
			total += 30
		case p.AmountNeededINR > s.Benefit.MaxAmountINR:
			// Too small to be useful. Penalise proportionally.
			ratio := float64(s.Benefit.MaxAmountINR) / float64(p.AmountNeededINR)
			total += int(15 * ratio)
		default:
			// Scheme floor is above what they need; they would be
			// over-borrowing.
			total += 5
		}
	}

	if s.Benefit.CollateralFree {
		total += 8
	}
	if s.Benefit.EffectiveInterestPercent > 0 && s.Benefit.EffectiveInterestPercent <= 7 {
		total += 8
	}
	if s.Benefit.SubsidyPercentMax > 0 {
		total += 6
	}
	// State schemes stack on top of central ones and are the ones applicants
	// are least likely to already know about.
	if s.Level == models.LevelState {
		total += 5
	}

	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}
	return total
}

// --- formatting helpers ---

func ageRangeText(min, max int) string {
	switch {
	case min > 0 && max > 0:
		return fmt.Sprintf("Age must be between %d and %d", min, max)
	case min > 0:
		return fmt.Sprintf("Minimum age %d", min)
	default:
		return fmt.Sprintf("Maximum age %d", max)
	}
}

// rupees formats an amount in the Indian grouping system, because
// "Rs 10,00,000" reads correctly to the user this is built for and
// "Rs 1,000,000" does not.
func rupees(n int) string {
	if n == 0 {
		return "Rs 0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	digits := fmt.Sprintf("%d", n)
	var out string
	if len(digits) <= 3 {
		out = digits
	} else {
		last3 := digits[len(digits)-3:]
		rest := digits[:len(digits)-3]

		var groups []string
		for len(rest) > 2 {
			groups = append([]string{rest[len(rest)-2:]}, groups...)
			rest = rest[:len(rest)-2]
		}
		if rest != "" {
			groups = append([]string{rest}, groups...)
		}
		out = strings.Join(groups, ",") + "," + last3
	}

	if neg {
		return "-Rs " + out
	}
	return "Rs " + out
}

func humanize(s string) string {
	return strings.ToUpper(s[:1]) + strings.ReplaceAll(s[1:], "_", " ")
}

// prettySchemeID turns "pm-svanidhi-1" into "PM SVANidhi 1" for the
// prerequisite message, which is read by the applicant, not a developer.
func prettySchemeID(id string) string {
	words := strings.Split(strings.ReplaceAll(id, "-", " "), " ")
	for i, w := range words {
		switch strings.ToLower(w) {
		case "pm", "pmegp", "kcc", "cgtmse", "nhfdc", "nlm", "sc", "st":
			words[i] = strings.ToUpper(w)
		case "svanidhi":
			words[i] = "SVANidhi"
		case "mudra":
			words[i] = "MUDRA"
		default:
			if w != "" {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
	}
	return strings.Join(words, " ")
}

func joinCategories(cs []models.Category) string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = strings.ToUpper(string(c))
	}
	return strings.Join(out, " / ")
}

func joinOccupations(os []models.Occupation) string {
	out := make([]string, len(os))
	for i, o := range os {
		out[i] = strings.ReplaceAll(string(o), "_", " ")
	}
	return strings.Join(out, ", ")
}

func genderAlternativeText(genders []string) string {
	if len(genders) == 0 {
		return ""
	}
	return fmt.Sprintf(", or any %s applicant", strings.Join(genders, "/"))
}

func containsCategory(list []models.Category, want models.Category) bool {
	for _, c := range list {
		if c == want {
			return true
		}
	}
	return false
}

func containsOccupation(list []models.Occupation, want models.Occupation) bool {
	for _, o := range list {
		if o == want {
			return true
		}
	}
	return false
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}
