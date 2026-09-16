package models

import (
	"strings"
	"time"
)

// Level separates centrally sponsored schemes from state ones. State schemes
// are where the real discovery value is -- central schemes are well indexed,
// state portals are not.
type Level string

const (
	LevelCentral Level = "central"
	LevelState   Level = "state"
)

// Kind is the financial instrument on offer.
type Kind string

const (
	KindLoan            Kind = "loan"
	KindSubsidy         Kind = "subsidy"
	KindCreditGuarantee Kind = "credit_guarantee"
	KindIncomeSupport   Kind = "income_support"
	KindInsurance       Kind = "insurance"
)

// Benefit is the money. This leads every scheme card in the UI, because
// "up to Rs 10,00,000, no collateral" is what makes someone read on.
type Benefit struct {
	MinAmountINR             int     `json:"minAmountInr,omitempty"`
	MaxAmountINR             int     `json:"maxAmountInr,omitempty"`
	SubsidyPercentMin        float64 `json:"subsidyPercentMin,omitempty"`
	SubsidyPercentMax        float64 `json:"subsidyPercentMax,omitempty"`
	EffectiveInterestPercent float64 `json:"effectiveInterestPercent,omitempty"`
	CollateralFree           bool    `json:"collateralFree"`
	TenureMonths             int     `json:"tenureMonths,omitempty"`
	Summary                  string  `json:"summary"`
}

// Eligibility is the structured rule set. Zero values mean "no constraint",
// which keeps schemes.json readable -- you only write the rules that bind.
type Eligibility struct {
	MinAge int `json:"minAge,omitempty"`
	MaxAge int `json:"maxAge,omitempty"`

	MaxAnnualIncomeINR int `json:"maxAnnualIncomeInr,omitempty"`

	// Empty slices mean "any".
	Categories  []Category   `json:"categories,omitempty"`
	Occupations []Occupation `json:"occupations,omitempty"`
	Genders     []string     `json:"genders,omitempty"`

	// Pointer so that an explicit 0-acre ceiling (landless only) is
	// distinguishable from "not specified".
	MaxLandHoldingAcres *float64 `json:"maxLandHoldingAcres,omitempty"`

	MinBusinessVintageMonths int `json:"minBusinessVintageMonths,omitempty"`
	MaxAnnualTurnoverINR     int `json:"maxAnnualTurnoverInr,omitempty"`

	RequiresAadhaar     bool `json:"requiresAadhaar,omitempty"`
	RequiresBankAccount bool `json:"requiresBankAccount,omitempty"`
	RequiresUdyam       bool `json:"requiresUdyam,omitempty"`
	RequiresDisability  bool `json:"requiresDisability,omitempty"`

	// Holding any of these disqualifies the applicant.
	ExcludesSchemeIDs []string `json:"excludesSchemeIds,omitempty"`

	// Schemes that must ALREADY have been availed and repaid to qualify.
	// Tiered credit is the common case: the second PM SVANidhi tranche is
	// only open to someone who repaid the first, and MUDRA Tarun Plus only
	// to someone who repaid Tarun. Without this, a first-time applicant gets
	// shown the largest tier and sent to a bank that will turn them away.
	RequiresSchemeIDs []string `json:"requiresSchemeIds,omitempty"`

	// Free-text caveats shown on the card. Used for rules too fuzzy to
	// encode, e.g. "bank assesses repayment capacity".
	Notes []string `json:"notes,omitempty"`
}

// Scheme is one catalog entry.
type Scheme struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	NameLocal map[string]string `json:"nameLocal,omitempty"`
	ShortDesc string            `json:"shortDesc"`
	Authority string            `json:"authority"`

	Level Level `json:"level"`
	// Empty means all-India. Otherwise lowercase state slugs.
	States []string `json:"states,omitempty"`

	Kind     Kind      `json:"kind"`
	Purposes []Purpose `json:"purposes"`

	Benefit     Benefit     `json:"benefit"`
	Eligibility Eligibility `json:"eligibility"`

	Documents []string `json:"documents,omitempty"`

	ApplyURL string `json:"applyUrl,omitempty"`
	// Domains that legitimately host this scheme's application or guidelines.
	// The fraud shield treats everything else as suspect.
	OfficialDomains []string `json:"officialDomains,omitempty"`
	SourceURLs      []string `json:"sourceUrls,omitempty"`

	// Seeds for the freshness check and for suppressing already-known schemes
	// from discovery results.
	SearchKeywords []string `json:"searchKeywords,omitempty"`

	CatalogVerifiedOn string `json:"catalogVerifiedOn,omitempty"`
	// True until a human has checked the amounts and rules against the
	// official source. The UI surfaces this; do not ship a submission with
	// these still set.
	NeedsVerification bool `json:"needsVerification,omitempty"`
}

// AppliesToState reports whether the scheme is available in the given state.
func (s Scheme) AppliesToState(state string) bool {
	if len(s.States) == 0 {
		return true
	}
	want := normalizeState(state)
	for _, st := range s.States {
		if normalizeState(st) == want {
			return true
		}
	}
	return false
}

// ServesPurpose reports whether the scheme can fund the given purpose.
func (s Scheme) ServesPurpose(p Purpose) bool {
	if len(s.Purposes) == 0 {
		return true
	}
	for _, sp := range s.Purposes {
		if sp == p {
			return true
		}
	}
	return false
}

func normalizeState(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
}

// MatchStatus is the verdict for one scheme against one profile.
type MatchStatus string

const (
	StatusEligible   MatchStatus = "eligible"
	StatusNearMiss   MatchStatus = "near_miss"
	StatusIneligible MatchStatus = "ineligible"
)

// Reason is one evaluated rule. Every card shows these, so the applicant can
// see *why* -- an opaque yes/no is not trustworthy when money is involved.
type Reason struct {
	Rule   string `json:"rule"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
	// Blocking rules can't be fixed by the applicant (age, category).
	// Non-blocking ones can (open a bank account, register on Udyam) and
	// become unlock steps.
	Blocking bool `json:"blocking"`
}

// Evidence is a single search result backing a freshness or fraud finding.
type Evidence struct {
	Title  string `json:"title"`
	Link   string `json:"link"`
	Source string `json:"source,omitempty"`
	Date   string `json:"date,omitempty"`
}

// FreshnessStatus is the outcome of the live news/search verification pass.
type FreshnessStatus string

const (
	FreshnessOK      FreshnessStatus = "ok"
	FreshnessChanged FreshnessStatus = "changed"
	FreshnessClosed  FreshnessStatus = "closed"
	FreshnessUnknown FreshnessStatus = "unknown"
)

// Freshness answers "is this scheme still open, today?" -- the thing a static
// scheme directory can never tell you.
type Freshness struct {
	Status    FreshnessStatus `json:"status"`
	Note      string          `json:"note,omitempty"`
	Evidence  []Evidence      `json:"evidence,omitempty"`
	CheckedAt time.Time       `json:"checkedAt"`
}

// Impostor is a non-official page ranking for a scheme name while soliciting
// applications -- typically a fee-charging lookalike.
type Impostor struct {
	Domain string `json:"domain"`
	Title  string `json:"title"`
	Link   string `json:"link"`
	Why    string `json:"why"`
}

// TrustReport is the fraud shield output for one scheme.
type TrustReport struct {
	ApplyURLOfficial   bool       `json:"applyUrlOfficial"`
	ApplyDomain        string     `json:"applyDomain,omitempty"`
	SuspectedImpostors []Impostor `json:"suspectedImpostors,omitempty"`
	CheckedAt          time.Time  `json:"checkedAt"`
}

// Match is a scheme plus the verdict and the live verification results.
type Match struct {
	Scheme      Scheme       `json:"scheme"`
	Status      MatchStatus  `json:"status"`
	Score       int          `json:"score"`
	Reasons     []Reason     `json:"reasons"`
	UnlockSteps []string     `json:"unlockSteps,omitempty"`
	Freshness   *Freshness   `json:"freshness,omitempty"`
	Trust       *TrustReport `json:"trust,omitempty"`
	Origin      string       `json:"origin"` // catalog | discovered
}

// Lead is a candidate scheme found by search that is not in the catalog --
// usually a state scheme. Leads are shown separately and never presented as
// verified eligibility.
type Lead struct {
	Title      string  `json:"title"`
	Link       string  `json:"link"`
	Snippet    string  `json:"snippet,omitempty"`
	Domain     string  `json:"domain"`
	Query      string  `json:"query,omitempty"`
	Language   string  `json:"language,omitempty"`
	Official   bool    `json:"official"`
	Confidence float64 `json:"confidence"`
}

// SearchUsage reports what the request cost, so the UI can show the credit
// burn and the demo can prove it ran from cache.
type SearchUsage struct {
	Calls      int `json:"calls"`
	CacheHits  int `json:"cacheHits"`
	LiveCalls  int `json:"liveCalls"`
	CreditsMax int `json:"creditsMax"`
}

// MatchResponse is the payload the frontend renders.
type MatchResponse struct {
	Profile     Profile     `json:"profile"`
	Eligible    []Match     `json:"eligible"`
	NearMisses  []Match     `json:"nearMisses"`
	Discovered  []Lead      `json:"discovered"`
	Usage       SearchUsage `json:"usage"`
	Warnings    []string    `json:"warnings,omitempty"`
	GeneratedAt time.Time   `json:"generatedAt"`
}
