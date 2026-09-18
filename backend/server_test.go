package aegiscortex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBindQuestionsCount(t *testing.T) {
	q, specs := BuildPipelineQuestions(PipelineRAG)
	if len(q) != 11 {
		t.Fatalf("expected 11 bound questions, got %d", len(q))
	}
	if len(specs) != 11 {
		t.Fatalf("expected 11 specs, got %d", len(specs))
	}
	fields := 0
	for _, s := range specs {
		if isFieldQuestion(s.Key) {
			fields++
		}
	}
	if fields != 4 {
		t.Fatalf("expected 4 field questions, got %d", fields)
	}
}

func TestPresetPayloadsDriveRouting(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	cases := []struct {
		id   string
		tier string
	}{
		{"gw_rag_injection", "TIER_0_BLOCK"},
		{"sde_hallucinated_date", "TIER_2_SURGICAL_FIELD_REPAIR"},
		{"rag_verified_fastpath", "TIER_1_VERIFIED_FASTPATH"},
		{"triage_auto_refund", "TIER_0_AUTO_EXEC"},
	}
	for _, tc := range cases {
		preset, ok := presetByID(tc.id)
		if !ok {
			t.Fatalf("missing preset %s", tc.id)
		}
		res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{
			ScenarioID: "",
			Context:    preset.State,
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if res.FinalRouteTier != tc.tier {
			t.Fatalf("%s: expected %s from payload, got %s (%s)", tc.id, tc.tier, res.FinalRouteTier, res.FinalDecision)
		}
		if len(res.Questions) != 11 {
			t.Fatalf("%s: expected 11 questions, got %d", tc.id, len(res.Questions))
		}
		if len(res.FieldVerifications) != 4 {
			t.Fatalf("%s: expected 4 field cards, got %d", tc.id, len(res.FieldVerifications))
		}
		if res.Mode != "LIVE_TYPESAFE_API" && res.LatencyMs > 90 {
			t.Fatalf("%s: simulator latency should be honest wall-clock, got %.0f ms", tc.id, res.LatencyMs)
		}
	}
}

func TestScenarioLabelCannotOverridePayload(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	inj, _ := presetByID("gw_rag_injection")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{
		ScenarioID: "rag_verified_fastpath",
		Context:    inj.State,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalRouteTier != "TIER_0_BLOCK" {
		t.Fatalf("injection payload labeled as fastpath should still block, got %s", res.FinalRouteTier)
	}
}

func TestHallucinatedDateFailsOnlyDateField(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	preset, _ := presetByID("sde_hallucinated_date")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{ScenarioID: "sde_hallucinated_date", Context: preset.State})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalRouteTier != "TIER_2_SURGICAL_FIELD_REPAIR" {
		t.Fatalf("got %s", res.FinalRouteTier)
	}
	var date FieldVerificationView
	locked := 0
	for _, f := range res.FieldVerifications {
		if f.Verified {
			locked++
		}
		if f.FieldName == "effective_or_notice_date" {
			date = f
		}
	}
	if date.Verified {
		t.Fatalf("notice date should fail, got %+v", date)
	}
	if locked < 2 {
		t.Fatalf("expected other fields to lock, locked=%d fields=%v", locked, res.FailedFields)
	}
}

func TestCompositeThresholdAffectsFastPath(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	engine.SetThresholds(PipelineThresholds{
		SecurityGateConfidence: 0.65,
		FieldVerifyConfidence:  0.75,
		RouterConfidence:       0.80,
		CompositePassThreshold: 99.5,
	})
	preset, _ := presetByID("rag_verified_fastpath")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{Context: preset.State})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalRouteTier != "TIER_2_SURGICAL_FIELD_REPAIR" {
		t.Fatalf("impossible composite gate should escalate, got %s score=%.1f", res.FinalRouteTier, res.CompositeScore)
	}
}

func TestEditedJSONWithoutInjectionUnblocks(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	ctx := map[string]any{
		"user_prompt": "Summarize the Q3 vendor onboarding notes from the retrieved confluence pages.",
		"retrieved_passages": []map[string]any{
			{"id": "doc_101", "text": "Q3 vendor onboarding requires SOC2 Type II certification and net-30 invoicing."},
		},
		"source_document": "Q3 vendor onboarding requires SOC2 Type II certification and net-30 invoicing.",
		"draft_reply":     "Q3 vendor onboarding requires SOC2 Type II certification and net-30 invoicing.",
	}
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{
		ScenarioID: "gw_rag_injection",
		Context:    ctx,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalRouteTier == "TIER_0_BLOCK" {
		t.Fatalf("cleaned payload should not block, got %s (%s)", res.FinalRouteTier, res.FinalDecision)
	}
}

func TestHTTPServerSecurityAndEvaluate(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	handler, err := NewServerHandler(engine)
	if err != nil {
		t.Fatal(err)
	}

	reqRoot := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	recRoot := httptest.NewRecorder()
	handler.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK {
		t.Fatalf("GET /api/state = %d", recRoot.Code)
	}
	body := recRoot.Body.String()
	if !strings.Contains(body, "rag_verified_fastpath") || strings.Contains(body, "cdn.tailwindcss.com") {
		t.Fatalf("expected JSON studio state, got %s", body)
	}
	if csp := recRoot.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") || strings.Contains(csp, "cdn.tailwindcss.com") {
		t.Fatalf("CSP not locked down: %q", recRoot.Header().Get("Content-Security-Policy"))
	}
	if recRoot.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing XFO")
	}

	preset, _ := presetByID("gw_rag_injection")
	payload, _ := json.Marshal(EvaluationRequest{Context: preset.State})
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate = %d %s", rec.Code, rec.Body.String())
	}
	var res EvaluationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.FinalRouteTier != "TIER_0_BLOCK" {
		t.Fatalf("got %s", res.FinalRouteTier)
	}

	bad := httptest.NewRequest(http.MethodPost, "/api/evaluate", strings.NewReader("{"))
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json = %d", badRec.Code)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/evaluate", nil)
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT = %d", putRec.Code)
	}

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, health)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", healthRec.Code)
	}
}

func TestInspectStateHallucination(t *testing.T) {
	preset, _ := presetByID("sde_hallucinated_date")
	sig := inspectState(preset.State)
	if !sig.HallucinatedDate {
		t.Fatalf("expected hallucinated date signal: %+v", sig)
	}
	if !sig.VendorSupported || !sig.AmountSupported {
		t.Fatalf("vendor/amount should be supported: %+v", sig)
	}
}

func TestListenAddrLocalAndVercel(t *testing.T) {
	t.Setenv("VERCEL", "")
	t.Setenv("VERCEL_ENV", "")
	t.Setenv("VERCEL_URL", "")
	t.Setenv("VERCEL_REGION", "")
	t.Setenv("AEGIS_ADDR", "")
	t.Setenv("AEGIS_PORT", "")
	t.Setenv("PORT", "")
	addr, err := ListenAddr()
	if err != nil || addr != defaultBindAddr {
		t.Fatalf("default addr = %q %v", addr, err)
	}

	t.Setenv("AEGIS_PORT", "9100")
	addr, err = ListenAddr()
	if err != nil || addr != "127.0.0.1:9100" {
		t.Fatalf("AEGIS_PORT addr = %q %v", addr, err)
	}

	t.Setenv("AEGIS_ADDR", "127.0.0.1:9200")
	addr, err = ListenAddr()
	if err != nil || addr != "127.0.0.1:9200" {
		t.Fatalf("AEGIS_ADDR addr = %q %v", addr, err)
	}

	t.Setenv("AEGIS_ADDR", "0.0.0.0:8090")
	if _, err := ListenAddr(); err == nil {
		t.Fatal("expected loopback policy to reject 0.0.0.0")
	}
	t.Setenv("AEGIS_ADDR", "")

	t.Setenv("PORT", "8080")
	addr, err = ListenAddr()
	if err != nil || addr != ":8080" {
		t.Fatalf("PORT addr = %q %v", addr, err)
	}

	t.Setenv("VERCEL", "1")
	t.Setenv("PORT", "8080")
	addr, err = ListenAddr()
	if err != nil || addr != ":8080" {
		t.Fatalf("vercel addr = %q %v", addr, err)
	}

	t.Setenv("PORT", "")
	addr, err = ListenAddr()
	if err != nil || addr != ":3001" {
		t.Fatalf("hosted without PORT = %q %v", addr, err)
	}
}

func TestVercelRejectsBrowserKey(t *testing.T) {
	t.Setenv("VERCEL", "1")
	engine := NewCortexEngineWithKey("")
	handler, err := NewServerHandler(engine)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/key", strings.NewReader(`{"api_key":"should-not-stick"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("hosted /api/key = %d %s", rec.Code, rec.Body.String())
	}
	if engine.HasAPIKey() {
		t.Fatal("hosted POST /api/key must not store a key")
	}

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, health)
	if healthRec.Code != http.StatusOK || !strings.Contains(healthRec.Body.String(), `"hosted":true`) {
		t.Fatalf("hosted healthz = %d %s", healthRec.Code, healthRec.Body.String())
	}
}

func TestLiveTypeSafeFanout(t *testing.T) {
	key := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY"))
	if key == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	engine := NewCortexEngineWithKey(key)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	want := map[string]string{
		"gw_rag_injection":      "TIER_0_BLOCK",
		"sde_hallucinated_date": "TIER_2_SURGICAL_FIELD_REPAIR",
		"rag_verified_fastpath": "TIER_1_VERIFIED_FASTPATH",
		"triage_auto_refund":    "TIER_0_AUTO_EXEC",
	}
	for id, tier := range want {
		preset, ok := presetByID(id)
		if !ok {
			t.Fatalf("missing preset %s", id)
		}
		res, err := engine.EvaluateRequest(ctx, EvaluationRequest{ScenarioID: id, Context: preset.State})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if res.FallbackUsed || res.Mode != "LIVE_TYPESAFE_API" {
			t.Fatalf("%s: expected live API, mode=%s fallback=%v err=%s", id, res.Mode, res.FallbackUsed, res.LiveError)
		}
		if res.FinalRouteTier != tier {
			t.Fatalf("%s: live route %s want %s (%s) fields=%v", id, res.FinalRouteTier, tier, res.FinalDecision, res.FailedFields)
		}
	}
}
