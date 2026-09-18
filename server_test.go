package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAegisCortexServerAndCascades(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	handler, err := NewServerHandler(engine)
	if err != nil {
		t.Fatalf("NewServerHandler failed: %v", err)
	}

	// 1. Verify GET / renders Go html/template with Tailwind + security headers
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET /, got %d", recRoot.Code)
	}
	if !strings.Contains(recRoot.Body.String(), "AEGIS") || !strings.Contains(recRoot.Body.String(), "cdn.tailwindcss.com") {
		t.Fatalf("expected rendered html/template with Tailwind CSS")
	}
	if csp := recRoot.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("missing strict Content-Security-Policy header: %q", csp)
	}
	if xfo := recRoot.Header().Get("X-Frame-Options"); xfo != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", xfo)
	}

	// 2. Verify all 4 scenarios produce expected 3-tier routing
	cases := []struct {
		scenarioID   string
		expectedTier string
	}{
		{"gw_rag_injection", "TIER_0_BLOCK"},
		{"sde_hallucinated_date", "TIER_2_SURGICAL_FIELD_REPAIR"},
		{"rag_verified_fastpath", "TIER_1_VERIFIED_FASTPATH"},
		{"triage_auto_refund", "TIER_0_AUTO_EXEC"},
	}

	for _, tc := range cases {
		body, _ := json.Marshal(EvaluationRequest{ScenarioID: tc.scenarioID})
		req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("scenario %s returned %d", tc.scenarioID, rec.Code)
		}
		var res EvaluationResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if res.FinalRouteTier != tc.expectedTier {
			t.Fatalf("scenario %s: expected tier %s, got %s", tc.scenarioID, tc.expectedTier, res.FinalRouteTier)
		}
		if len(res.Questions) != 11 {
			t.Fatalf("scenario %s: expected 11 fan-out questions, got %d", tc.scenarioID, len(res.Questions))
		}
	}
}
