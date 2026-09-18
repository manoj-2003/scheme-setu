package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/manoj-2003/scheme-setu/internal/catalog"
	"github.com/manoj-2003/scheme-setu/internal/fraud"
	"github.com/manoj-2003/scheme-setu/internal/freshness"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

// newTestHandler builds the pipeline against a throwaway cache with no API
// key, so tests never touch the network or spend credits.
func newTestHandler(t *testing.T) (*echo.Echo, *Handler) {
	t.Helper()

	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	client, err := serp.New(serp.Options{
		Mode:      serp.ModeCacheOnly,
		CachePath: filepath.Join(t.TempDir(), "cache.sqlite"),
	})
	if err != nil {
		t.Fatalf("serp client: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	h := &Handler{
		Catalog:             cat,
		Client:              client,
		Fresh:               freshness.New(client),
		Fraud:               fraud.New(client, cat.OfficialDomains()),
		DiscoveryLangs:      []string{"en"},
		MaxDiscoveryQueries: 2,
		VerifyTopN:          1,
	}

	e := echo.New()
	// No validator is registered, so bind-only paths are exercised here; the
	// server wires the real one in cmd/server.
	e.Validator = passthroughValidator{}
	h.Register(e)
	return e, h
}

type passthroughValidator struct{}

func (passthroughValidator) Validate(any) error { return nil }

func post(t *testing.T, e *echo.Echo, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// A nil slice in Go marshals to null, which crashed the frontend on
// response.nearMisses.length. Empty and absent mean the same thing to a
// caller, so the API must always send an array.
func TestMatchNeverReturnsNullForListFields(t *testing.T) {
	e, _ := newTestHandler(t)

	// A salaried homeowner wanting rooftop solar: matches one scheme and
	// produces no near misses, which is the shape that triggered the bug.
	rec := post(t, e, `{
		"state": "gujarat", "age": 52, "gender": "male",
		"category": "general", "occupation": "salaried",
		"annualIncomeInr": 900000, "purpose": "rooftop_solar",
		"amountNeededInr": 200000,
		"hasAadhaar": true, "hasBankAccount": true
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	// Check the raw JSON, not the decoded struct: encoding/json turns both
	// null and [] back into a nil slice, so a struct-level assertion would
	// pass even with the bug present.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	for _, field := range []string{"eligible", "nearMisses", "discovered", "warnings"} {
		val, ok := raw[field]
		if !ok {
			t.Errorf("%s missing from response", field)
			continue
		}
		if string(val) == "null" {
			t.Errorf("%s is null; clients doing .%s.length will throw", field, field)
		}
		if !strings.HasPrefix(string(val), "[") {
			t.Errorf("%s is not a JSON array: %s", field, val)
		}
	}
}

func TestMatchReturnsExplainedResults(t *testing.T) {
	e, _ := newTestHandler(t)

	rec := post(t, e, `{
		"state": "tamil_nadu", "age": 34, "gender": "female",
		"category": "obc", "occupation": "street_vendor",
		"annualIncomeInr": 180000, "purpose": "working_capital",
		"amountNeededInr": 50000, "businessVintageMonths": 30,
		"hasAadhaar": true, "hasBankAccount": true, "language": "ta"
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Eligible []struct {
			Scheme    struct{ ID string }
			Status    string
			Reasons   []struct{ Rule string }
			Freshness *struct{ Status string }
		}
		Usage struct{ LiveCalls int }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got.Eligible) == 0 {
		t.Fatal("a street vendor needing working capital should match at least one scheme")
	}
	for _, m := range got.Eligible {
		if len(m.Reasons) == 0 {
			t.Errorf("%s has no reasons; the verdict is not explainable", m.Scheme.ID)
		}
		// Every card must carry a freshness verdict, even if only to say it
		// was not verified. Silence would imply it had been checked.
		if m.Freshness == nil || m.Freshness.Status == "" {
			t.Errorf("%s has no freshness status", m.Scheme.ID)
		}
	}

	// Cache-only mode must never spend a credit.
	if got.Usage.LiveCalls != 0 {
		t.Errorf("cache-only mode spent %d live credits", got.Usage.LiveCalls)
	}
}

func TestHealthAndMeta(t *testing.T) {
	e, _ := newTestHandler(t)

	for _, path := range []string{"/healthz", "/api/v1/meta", "/api/v1/usage", "/api/v1/schemes"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s -> %d", path, rec.Code)
		}
	}
}
