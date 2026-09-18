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
	req.Header.Set("Content-Type", "application/json")
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
	bad.Header.Set("Content-Type", "application/json")
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

	boot := httptest.NewRequest(http.MethodGet, "/api/boot?case=sde_hallucinated_date", nil)
	bootRec := httptest.NewRecorder()
	handler.ServeHTTP(bootRec, boot)
	if bootRec.Code != http.StatusOK {
		t.Fatalf("boot = %d %s", bootRec.Code, bootRec.Body.String())
	}
	if !strings.Contains(bootRec.Body.String(), `"scenario_id":"sde_hallucinated_date"`) {
		t.Fatalf("boot permalink missed case: %s", bootRec.Body.String())
	}

	btReq := httptest.NewRequest(http.MethodPost, "/api/backtest", strings.NewReader(`{"synthetic":4,"monthly_volume":10000000}`))
	btReq.Header.Set("Content-Type", "application/json")
	btRec := httptest.NewRecorder()
	handler.ServeHTTP(btRec, btReq)
	if btRec.Code != http.StatusOK || !strings.Contains(btRec.Body.String(), `"turns":4`) {
		t.Fatalf("backtest = %d %s", btRec.Code, btRec.Body.String())
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
	req.Header.Set("Content-Type", "application/json")
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

func TestSchemaCompilerAndSurgicalRepair(t *testing.T) {
	schema := map[string]any{
		"title": "MSAExtraction",
		"type":  "object",
		"properties": map[string]any{
			"vendor_name":          map[string]any{"type": "string"},
			"contract_value_usd":   map[string]any{"type": "number"},
			"effective_date":       map[string]any{"type": "string"},
			"notice_deadline_date": map[string]any{"type": "string"},
			"auto_renews":          map[string]any{"type": "boolean"},
		},
	}
	compiled, err := CompileJSONSchema(schema)
	if err != nil || len(compiled.Fields) != 5 {
		t.Fatalf("compile = %+v %v", compiled, err)
	}
	if !strings.Contains(compiled.Stub, "verify_field_notice_deadline_date") {
		t.Fatalf("stub missing field noul: %s", compiled.Stub)
	}

	engine := NewCortexEngineWithKey("")
	preset, _ := presetByID("sde_hallucinated_date")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{
		ScenarioID: "sde_hallucinated_date",
		Context:    preset.State,
		Schema:     schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.QuestionCount < 12 {
		t.Fatalf("custom schema should expand fan-out, got %d", res.QuestionCount)
	}
	if res.Surgical == nil || !res.Surgical.Executed {
		t.Fatalf("expected surgical patch, got %+v", res.Surgical)
	}
	found := false
	for _, p := range res.Surgical.Patches {
		if strings.Contains(p.Field, "notice") || strings.Contains(p.Field, "date") {
			if stringify(p.After) != "10/02/2026" {
				t.Fatalf("notice repair = %v want 10/02/2026 method=%s", p.After, p.Method)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("missing notice date patch: %+v", res.Surgical.Patches)
	}
}

func TestSurgicalRepairUsesPresetStateAndLeavesLockedDates(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{ScenarioID: "sde_hallucinated_date"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Surgical == nil {
		t.Fatal("expected surgical patch from scenario_id alone")
	}
	var notice, effective *FieldPatch
	for i := range res.Surgical.Patches {
		p := &res.Surgical.Patches[i]
		if p.Field == "notice_deadline_date" {
			notice = p
		}
		if p.Field == "effective_date" {
			effective = p
		}
	}
	if notice == nil || stringify(notice.After) != "10/02/2026" {
		t.Fatalf("notice patch = %+v", notice)
	}
	if effective != nil {
		t.Fatalf("locked effective_date must not be spliced: %+v", effective)
	}
	if stringify(res.Surgical.LockedJSON["effective_date"]) != "11/01/2025" {
		t.Fatalf("locked JSON dropped effective_date: %+v", res.Surgical.LockedJSON)
	}
}

func TestScoreLabelAndPartialThreshold(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	before := engine.GetThresholds()
	engine.SetThresholds(PipelineThresholds{SecurityGateConfidence: 0.70})
	after := engine.GetThresholds()
	if after.CompositePassThreshold != before.CompositePassThreshold {
		t.Fatalf("partial JSON must not zero composite: before=%.1f after=%.1f", before.CompositePassThreshold, after.CompositePassThreshold)
	}
	if after.SecurityGateConfidence != 0.70 {
		t.Fatalf("security gate not updated: %+v", after)
	}

	preset, _ := presetByID("gw_rag_injection")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{Context: preset.State})
	if err != nil {
		t.Fatal(err)
	}
	var harm QuestionView
	for _, q := range res.Questions {
		if q.ID == qHarmSeverity {
			harm = q
		}
	}
	if !strings.Contains(harm.SelectedChoice, "Severe") && !strings.Contains(harm.SelectedChoice, "Moderate") {
		t.Fatalf("policy_harm_severity label = %q", harm.SelectedChoice)
	}
}

func TestReadOnlyBootDoesNotMutateFlywheel(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	preset, _ := presetByID("rag_verified_fastpath")
	_, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{
		ScenarioID: "rag_verified_fastpath",
		Context:    preset.State,
		ReadOnly:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if engine.GetFlywheel().DistilledGoldenExamples != 0 {
		t.Fatalf("read-only boot mutated flywheel: %+v", engine.GetFlywheel())
	}
}

func TestRejectPlainTextPOSTAndPublicKey(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	handler, err := NewServerHandler(engine)
	if err != nil {
		t.Fatal(err)
	}
	plain := httptest.NewRequest(http.MethodPost, "/api/evaluate", strings.NewReader(`{"context":{}}`))
	plain.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, plain)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain POST = %d %s", rec.Code, rec.Body.String())
	}

	t.Setenv("PORT", "8090")
	t.Setenv("VERCEL", "")
	t.Setenv("VERCEL_ENV", "")
	t.Setenv("VERCEL_URL", "")
	t.Setenv("VERCEL_REGION", "")
	keyReq := httptest.NewRequest(http.MethodPost, "/api/key", strings.NewReader(`{"api_key":"x"}`))
	keyReq.Header.Set("Content-Type", "application/json")
	keyReq.RemoteAddr = "203.0.113.9:4400"
	keyRec := httptest.NewRecorder()
	handler.ServeHTTP(keyRec, keyReq)
	if keyRec.Code != http.StatusForbidden {
		t.Fatalf("public bind /api/key = %d %s", keyRec.Code, keyRec.Body.String())
	}
}

func TestSolverMeetsEscapeSLA(t *testing.T) {
	engine := NewCortexEngineWithKey("")
	for _, id := range []string{"gw_rag_injection", "sde_hallucinated_date", "rag_verified_fastpath", "triage_auto_refund"} {
		preset, _ := presetByID(id)
		if _, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{ScenarioID: id, Context: preset.State}); err != nil {
			t.Fatal(err)
		}
	}
	solved, ok := engine.solveAndRecommend(0.001)
	if !ok || solved.Samples < 4 {
		t.Fatalf("solver = %+v ok=%v", solved, ok)
	}
	if solved.EscapeRate > 0.001 {
		t.Fatalf("escape rate %.4f exceeds SLA", solved.EscapeRate)
	}
}

func TestInferExtraFieldsAndBacktest(t *testing.T) {
	ctx := map[string]any{
		"source_document": "Governing law is the State of Delaware. Payment terms are net 30.",
		"mini_model_extraction": map[string]any{
			"vendor_name":          "Northwind Analytics LLC",
			"contract_value_usd":   120000,
			"effective_date":       "11/01/2025",
			"notice_deadline_date": "09/15/2026",
			"auto_renews":          true,
			"governing_law":        "State of Delaware",
			"payment_terms_days":   30,
		},
	}
	extra := InferExtraFields(ctx)
	if len(extra) < 2 {
		t.Fatalf("expected extra fields for governing_law / payment_terms, got %+v", extra)
	}
	engine := NewCortexEngineWithKey("")
	res, err := engine.EvaluateRequest(context.Background(), EvaluationRequest{Context: ctx})
	if err != nil {
		t.Fatal(err)
	}
	if res.QuestionCount < 12 {
		t.Fatalf("extra payload keys should expand fan-out, got %d", res.QuestionCount)
	}
	if res.AuditHash == "" || res.Scorer != "aegis_calibrated_simulator" {
		t.Fatalf("audit/scorer missing: hash=%s scorer=%s", res.AuditHash, res.Scorer)
	}

	report, err := engine.RunBacktest(context.Background(), BacktestRequest{Synthetic: 8, MonthlyVolume: 10_000_000})
	if err != nil || report.Turns != 8 {
		t.Fatalf("backtest = %+v %v", report, err)
	}
	if report.MonthlyBaselineUSD <= report.MonthlyAegisUSD {
		t.Fatalf("expected modeled monthly savings: %+v", report)
	}
}

func TestEngineKeyPrefersAegis(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "legacy")
	t.Setenv("AEGIS_ENGINE_KEY", "first-party")
	if got := EngineKeyFromEnv(); got != "first-party" {
		t.Fatalf("EngineKeyFromEnv = %q", got)
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
