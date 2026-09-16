// Package models holds the shapes shared by the matching engine, the
// discovery layer and the HTTP handlers.
package models

import "strings"

// Category is the social category most reservation-linked schemes key off.
type Category string

const (
	CategoryGeneral  Category = "general"
	CategoryOBC      Category = "obc"
	CategorySC       Category = "sc"
	CategoryST       Category = "st"
	CategoryEWS      Category = "ews"
	CategoryMinority Category = "minority"
)

// Occupation buckets an applicant into the groups schemes actually target.
type Occupation string

const (
	OccupationFarmer          Occupation = "farmer"
	OccupationStreetVendor    Occupation = "street_vendor"
	OccupationArtisan         Occupation = "artisan"
	OccupationMicroEnterprise Occupation = "micro_enterprise"
	OccupationSelfEmployed    Occupation = "self_employed"
	OccupationSalaried        Occupation = "salaried"
	OccupationStudent         Occupation = "student"
	OccupationUnemployed      Occupation = "unemployed"
)

// Purpose is what the money is for. Schemes are indexed by purpose so a
// profile only triggers the searches that could possibly match it, which
// matters when the whole month's budget is 250 credits.
type Purpose string

const (
	PurposeWorkingCapital  Purpose = "working_capital"
	PurposeEquipment       Purpose = "equipment"
	PurposeNewBusiness     Purpose = "new_business"
	PurposeElectricVehicle Purpose = "electric_vehicle"
	PurposeRooftopSolar    Purpose = "rooftop_solar"
	PurposeEducation       Purpose = "education"
	PurposeLivestock       Purpose = "livestock"
	PurposeFoodProcessing  Purpose = "food_processing"
	PurposeIncomeSupport   Purpose = "income_support"
)

// Profile is the wizard payload. Everything here is either a hard eligibility
// input or a readiness flag that drives an "unlock step" hint.
type Profile struct {
	State    string `json:"state" validate:"required"`
	District string `json:"district,omitempty"`

	Age    int    `json:"age" validate:"required,gte=16,lte=100"`
	Gender string `json:"gender,omitempty" validate:"omitempty,oneof=male female other"`

	AnnualIncomeINR int        `json:"annualIncomeInr" validate:"gte=0"`
	Category        Category   `json:"category" validate:"required,oneof=general obc sc st ews minority"`
	Occupation      Occupation `json:"occupation" validate:"required,oneof=farmer street_vendor artisan micro_enterprise self_employed salaried student unemployed"`

	LandHoldingAcres float64 `json:"landHoldingAcres" validate:"gte=0"`

	Purpose         Purpose `json:"purpose" validate:"required,oneof=working_capital equipment new_business electric_vehicle rooftop_solar education livestock food_processing income_support"`
	AmountNeededINR int     `json:"amountNeededInr" validate:"gte=0"`

	// Readiness flags. These rarely disqualify anyone outright, but they are
	// what produce the "you are one step away" hints -- the most actionable
	// part of the output for a first-time applicant.
	HasAadhaar     bool `json:"hasAadhaar"`
	HasBankAccount bool `json:"hasBankAccount"`
	HasUdyam       bool `json:"hasUdyam"`
	HasDisability  bool `json:"hasDisability"`

	BusinessVintageMonths int `json:"businessVintageMonths" validate:"gte=0"`
	AnnualTurnoverINR     int `json:"annualTurnoverInr" validate:"gte=0"`

	// Scheme IDs already availed. Several schemes are mutually exclusive
	// (a second PM SVANidhi tranche, for instance, depends on repaying the
	// first), so we need to know.
	ExistingSchemeIDs []string `json:"existingSchemeIds,omitempty"`

	// Language tag passed to SerpApi as hl=. Defaults to "en".
	Language string `json:"language,omitempty"`
}

// Lang returns the hl= value to use for this profile.
func (p Profile) Lang() string {
	if l := strings.TrimSpace(p.Language); l != "" {
		return strings.ToLower(l)
	}
	return "en"
}

// HasAvailed reports whether the applicant already holds the given scheme.
func (p Profile) HasAvailed(schemeID string) bool {
	for _, id := range p.ExistingSchemeIDs {
		if strings.EqualFold(id, schemeID) {
			return true
		}
	}
	return false
}
