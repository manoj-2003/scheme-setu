// Package handlers exposes the matching pipeline over HTTP.
package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/manoj-2003/scheme-setu/internal/catalog"
	"github.com/manoj-2003/scheme-setu/internal/discovery"
	"github.com/manoj-2003/scheme-setu/internal/fraud"
	"github.com/manoj-2003/scheme-setu/internal/freshness"
	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/rules"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// Handler holds the pipeline dependencies.
type Handler struct {
	Catalog *catalog.Catalog
	Client  *serp.Client
	Fresh   *freshness.Checker
	Fraud   *fraud.Checker

	// DiscoveryLangs are the hl= values used to hunt for state schemes.
	DiscoveryLangs []string
	// MaxDiscoveryQueries caps SerpApi calls spent on discovery per request.
	MaxDiscoveryQueries int
	// VerifyTopN caps how many matches get a live fraud check per request.
	VerifyTopN int
	// RequestTimeout bounds the whole fan-out.
	RequestTimeout time.Duration
}

// Register wires the routes onto an Echo instance.
func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.Health)

	v1 := e.Group("/api/v1")
	v1.POST("/match", h.Match)
	v1.GET("/schemes", h.Schemes)
	v1.GET("/meta", h.Meta)
	v1.GET("/usage", h.Usage)
}

// Health is a liveness probe.
func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "ok",
		"schemes": h.Catalog.Len(),
		"mode":    string(h.Client.Mode()),
	})
}

// Match is the core endpoint: profile in, ranked and verified schemes out.
func (h *Handler) Match(c echo.Context) error {
	var profile models.Profile
	if err := c.Bind(&profile); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid profile payload: "+err.Error())
	}
	if err := c.Validate(&profile); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	timeout := h.RequestTimeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
	defer cancel()

	// Narrow the catalog before spending anything: state, then purpose.
	candidates := h.Catalog.Candidates(profile)
	eligible, nearMisses := rules.EvaluateAll(profile, candidates)

	var (
		mu       sync.Mutex
		warnings []string
		found    discovery.Result
		wg       sync.WaitGroup
	)

	addWarnings := func(ws ...string) {
		if len(ws) == 0 {
			return
		}
		mu.Lock()
		warnings = append(warnings, ws...)
		mu.Unlock()
	}

	// The three search-backed passes are independent, so run them together.
	// Freshness and fraud enrich what we already matched; discovery looks for
	// what we never had.
	wg.Add(3)

	go func() {
		defer wg.Done()
		addWarnings(h.Fresh.CheckAll(ctx, eligible)...)
	}()

	go func() {
		defer wg.Done()
		addWarnings(h.checkTrust(ctx, eligible)...)
	}()

	go func() {
		defer wg.Done()
		found = discovery.Run(ctx, h.Client, profile, discovery.Options{
			Languages:     h.DiscoveryLangs,
			MaxQueries:    h.MaxDiscoveryQueries,
			KnownKeywords: h.Catalog.Keywords(),
		})
	}()

	wg.Wait()
	addWarnings(found.Warnings...)

	if unverified := h.Catalog.Unverified(); len(unverified) > 0 {
		addWarnings("Some catalog entries are still marked needsVerification: amounts and rules shown for them have not been checked against the official source.")
	}

	calls, hits, live, maxCredits := h.Client.Usage()

	return c.JSON(http.StatusOK, models.MatchResponse{
		Profile:    profile,
		Eligible:   eligible,
		NearMisses: nearMisses,
		Discovered: found.Leads,
		Usage: models.SearchUsage{
			Calls:      calls,
			CacheHits:  hits,
			LiveCalls:  live,
			CreditsMax: maxCredits,
		},
		Warnings:    warnings,
		GeneratedAt: time.Now(),
	})
}

// checkTrust runs the fraud shield over the top matches.
func (h *Handler) checkTrust(ctx context.Context, matches []models.Match) []string {
	limit := h.VerifyTopN
	if limit <= 0 {
		limit = 3
	}
	if limit > len(matches) {
		limit = len(matches)
	}

	var (
		mu       sync.Mutex
		warnings []string
		wg       sync.WaitGroup
	)

	for i := range matches[:limit] {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			report, err := h.Fraud.Check(ctx, matches[i].Scheme)
			if err != nil {
				mu.Lock()
				warnings = append(warnings, "trust check failed for "+matches[i].Scheme.ID+": "+err.Error())
				mu.Unlock()
				return
			}
			matches[i].Trust = report
		}(i)
	}
	wg.Wait()

	return warnings
}

// Schemes returns the whole catalog, for browsing and for debugging the rule
// set without running a match.
func (h *Handler) Schemes(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"count":      h.Catalog.Len(),
		"schemes":    h.Catalog.All(),
		"unverified": h.Catalog.Unverified(),
	})
}

// Meta returns the enumerations the wizard needs to build its dropdowns, so
// the frontend never hardcodes a list that can drift from the Go types.
func (h *Handler) Meta(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"states": discovery.States(),
		"categories": []models.Category{
			models.CategoryGeneral, models.CategoryOBC, models.CategorySC,
			models.CategoryST, models.CategoryEWS, models.CategoryMinority,
		},
		"occupations": []models.Occupation{
			models.OccupationFarmer, models.OccupationStreetVendor,
			models.OccupationArtisan, models.OccupationMicroEnterprise,
			models.OccupationSelfEmployed, models.OccupationSalaried,
			models.OccupationStudent, models.OccupationUnemployed,
		},
		"purposes": []models.Purpose{
			models.PurposeWorkingCapital, models.PurposeEquipment,
			models.PurposeNewBusiness, models.PurposeElectricVehicle,
			models.PurposeRooftopSolar, models.PurposeEducation,
			models.PurposeLivestock, models.PurposeFoodProcessing,
			models.PurposeIncomeSupport,
		},
		"languages": []map[string]string{
			{"code": "en", "label": "English"},
			{"code": "hi", "label": "हिन्दी"},
			{"code": "ta", "label": "தமிழ்"},
			{"code": "te", "label": "తెలుగు"},
			{"code": "bn", "label": "বাংলা"},
			{"code": "mr", "label": "मराठी"},
			{"code": "gu", "label": "ગુજરાતી"},
			{"code": "kn", "label": "ಕನ್ನಡ"},
		},
	})
}

// Usage reports SerpApi spend for this process. The demo uses this to show it
// is running off the committed cache rather than burning free-tier credits.
func (h *Handler) Usage(c echo.Context) error {
	calls, hits, live, maxCredits := h.Client.Usage()
	return c.JSON(http.StatusOK, map[string]any{
		"mode":             string(h.Client.Mode()),
		"searchCalls":      calls,
		"cacheHits":        hits,
		"liveCalls":        live,
		"creditCeiling":    maxCredits,
		"cachedResponses":  h.Client.CachedResponses(c.Request().Context()),
		"discoveryLangs":   h.DiscoveryLangs,
		"maxQueriesPerRun": h.MaxDiscoveryQueries,
	})
}
