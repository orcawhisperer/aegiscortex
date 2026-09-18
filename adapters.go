package main

import (
	"context"
	"strings"
)

// ScenarioPreset adapts PresetCase for the HTTP API and html/template.
type ScenarioPreset struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Badge       string         `json:"badge"`
	Description string         `json:"description"`
	Pipeline    PipelineID     `json:"pipeline"`
	Context     map[string]any `json:"context"`
}

// PipelineThresholds is the slider binding for the studio.
type PipelineThresholds struct {
	SecurityGateConfidence float64 `json:"security_gate_confidence"`
	FieldVerifyConfidence  float64 `json:"field_verify_confidence"`
	RouterConfidence       float64 `json:"router_confidence"`
	CompositePassThreshold float64 `json:"composite_pass_threshold"`
}

// EvaluationRequest is POST /api/evaluate.
type EvaluationRequest struct {
	ScenarioID string         `json:"scenario_id"`
	Context    map[string]any `json:"context"`
}

// FieldVerificationView is one card in the per-field gate.
type FieldVerificationView struct {
	FieldName  string  `json:"field_name"`
	QuestionID string  `json:"question_id"`
	YesProb    float64 `json:"yes_prob"`
	NoProb     float64 `json:"no_prob"`
	Confidence float64 `json:"confidence"`
	Verified   bool    `json:"verified"`
	Action     string  `json:"action"`
}

// QuestionView is one cell in the 11-question matrix.
type QuestionView struct {
	ID             string             `json:"id"`
	Stage          string             `json:"stage"`
	Type           string             `json:"type"`
	Prompt         string             `json:"prompt"`
	SelectedChoice string             `json:"selected_choice"`
	TopProbability float64            `json:"top_probability"`
	Confidence     float64            `json:"confidence"`
	AutoExecutable bool               `json:"auto_executable"`
	RouteReason    string             `json:"route_reason"`
	Probabilities  map[string]float64 `json:"probabilities,omitempty"`
}

// CascadeEconomicsView compares naive frontier, legacy router, and AegisCortex.
type CascadeEconomicsView struct {
	NaiveFrontierCostUSD   float64 `json:"naive_frontier_cost_usd"`
	NaiveFrontierLatencyMs float64 `json:"naive_frontier_latency_ms"`
	LegacyRouterCostUSD    float64 `json:"legacy_router_cost_usd"`
	LegacyRouterLatencyMs  float64 `json:"legacy_router_latency_ms"`
	AegisControlCostUSD    float64 `json:"aegis_control_cost_usd"`
	AegisControlLatencyMs  float64 `json:"aegis_control_latency_ms"`
	AegisBlendedCostUSD    float64 `json:"aegis_blended_cost_usd"`
	AegisTotalLatencyMs    float64 `json:"aegis_total_latency_ms"`
	ReferenceJevP50Ms      float64 `json:"reference_jev_p50_ms"`
	CostSavingsPercent     float64 `json:"cost_savings_percent"`
}

// FlywheelTelemetryView is the calibration flywheel for the studio.
type FlywheelTelemetryView struct {
	DistilledGoldenExamples  int     `json:"distilled_golden_examples"`
	Tier0BlockedOrAuto       int     `json:"tier0_blocked_or_auto"`
	Tier2SurgicalEscalations int     `json:"tier2_surgical_escalations"`
	GuardrailsBlocked        int     `json:"guardrails_blocked"`
	FastPathApproved         int     `json:"fast_path_approved"`
	AutoExecuted             int     `json:"auto_executed"`
	ExpectedCalibrationECE   float64 `json:"expected_calibration_ece"`
	RecommendedActGate       float64 `json:"recommended_act_gate"`
	RecommendedBlockProb     float64 `json:"recommended_block_prob"`
	RecommendedVerifyMin     float64 `json:"recommended_verify_min"`
	RecommendedComposite     float64 `json:"recommended_composite"`
}

// HistoryEntry is a compact flywheel row.
type HistoryEntry struct {
	Timestamp      string  `json:"timestamp"`
	ScenarioID     string  `json:"scenario_id"`
	FinalRouteTier string  `json:"final_route_tier"`
	Mode           string  `json:"mode"`
	LatencyMs      float64 `json:"latency_ms"`
}

// EvaluationResponse is returned by EvaluateRequest and embedded in the page boot payload.
type EvaluationResponse struct {
	Mode               string                  `json:"mode"`
	FallbackUsed       bool                    `json:"fallback_used"`
	LiveError          string                  `json:"live_error,omitempty"`
	ScenarioID         string                  `json:"scenario_id"`
	Pipeline           PipelineID              `json:"pipeline"`
	RequestID          string                  `json:"request_id"`
	FinalRouteTier     string                  `json:"final_route_tier"`
	FinalDecision      string                  `json:"final_decision"`
	SelectedSkill      string                  `json:"selected_skill"`
	FailedFields       []string                `json:"failed_fields"`
	CompositeScore     float64                 `json:"composite_score"`
	CompositeRisk      float64                 `json:"composite_risk"`
	LatencyMs          float64                 `json:"latency_ms"`
	InputTokens        int                     `json:"input_tokens"`
	Cascade            CascadeEconomicsView    `json:"cascade"`
	FieldVerifications []FieldVerificationView `json:"field_verifications"`
	Questions          []QuestionView          `json:"questions"`
	Flywheel           FlywheelTelemetryView   `json:"flywheel"`
	History            []HistoryEntry          `json:"history"`
}

// NewCortexEngineWithKey constructs a CortexEngine and optionally sets a key.
func NewCortexEngineWithKey(key string) *CortexEngine {
	e := NewCortexEngine()
	if strings.TrimSpace(key) != "" {
		e.SetAPIKey(key)
	}
	return e
}

// HasAPIKey reports whether a live TypeSafe key is in memory.
func (e *CortexEngine) HasAPIKey() bool {
	return e.HasLiveKey()
}

// GetPresets returns the four built-in scenarios.
func (e *CortexEngine) GetPresets() []ScenarioPreset {
	raw := DefaultPresets()
	out := make([]ScenarioPreset, 0, len(raw))
	for _, p := range raw {
		out = append(out, ScenarioPreset{
			ID:          p.ID,
			Title:       p.Title,
			Badge:       p.Badge,
			Description: p.Description,
			Pipeline:    p.Pipeline,
			Context:     p.State,
		})
	}
	return out
}

// GetThresholds maps internal gates onto slider values.
func (e *CortexEngine) GetThresholds() PipelineThresholds {
	t := e.snapshotThresholds()
	return PipelineThresholds{
		SecurityGateConfidence: t.GuardrailBlockProb,
		FieldVerifyConfidence:  t.CascadeVerifyMin,
		RouterConfidence:       t.ChoiceActConfidence,
		CompositePassThreshold: t.CompositePassScore,
	}
}

// SetThresholds updates gates from the studio sliders.
func (e *CortexEngine) SetThresholds(t PipelineThresholds) {
	e.UpdateThresholds(GateThresholds{
		GuardrailBlockProb:  t.SecurityGateConfidence,
		CascadeVerifyMin:    t.FieldVerifyConfidence,
		ChoiceActConfidence: t.RouterConfidence,
		ReviewMinConfidence: 0.50,
		CompositePassScore:  t.CompositePassThreshold,
	})
}

// GetFlywheel returns flywheel stats for the studio.
func (e *CortexEngine) GetFlywheel() FlywheelTelemetryView {
	m := e.snapshotMetrics()
	return FlywheelTelemetryView{
		DistilledGoldenExamples:  m.TotalRequests,
		Tier0BlockedOrAuto:       m.GuardrailsBlocked + m.AutoExecuted,
		Tier2SurgicalEscalations: m.HallucinationsCaught + m.ReasoningEscalations,
		GuardrailsBlocked:        m.GuardrailsBlocked,
		FastPathApproved:         m.FastPathApproved,
		AutoExecuted:             m.AutoExecuted,
		ExpectedCalibrationECE:   m.ExpectedCalibrationECE,
		RecommendedActGate:       m.RecommendedActGate,
		RecommendedBlockProb:     m.RecommendedBlockProb,
		RecommendedVerifyMin:     m.RecommendedVerifyMin,
		RecommendedComposite:     m.RecommendedComposite,
	}
}

func (e *CortexEngine) historyView(limit int) []HistoryEntry {
	hist := e.snapshotHistory()
	if limit <= 0 || limit > len(hist) {
		limit = len(hist)
	}
	out := make([]HistoryEntry, 0, limit)
	for i := 0; i < limit; i++ {
		o := hist[i]
		out = append(out, HistoryEntry{
			Timestamp:      o.Timestamp,
			ScenarioID:     string(o.Pipeline),
			FinalRouteTier: routeTierFromVerdict(o.FinalVerdict),
			Mode:           o.Mode,
			LatencyMs:      float64(o.Economics.WallClockLatencyMs),
		})
	}
	return out
}

// EvaluateRequest runs the engine and projects a studio response.
// Route tier and field cards come from the outcome, never from scenario_id.
func (e *CortexEngine) EvaluateRequest(ctx context.Context, req EvaluationRequest) (EvaluationResponse, error) {
	pipeline := PipelineRAG
	statePayload := req.Context

	if preset, ok := presetByID(req.ScenarioID); ok {
		pipeline = preset.Pipeline
		if len(statePayload) == 0 {
			statePayload = preset.State
		}
	}

	outcome, err := e.Evaluate(ctx, pipeline, statePayload, e.HasLiveKey())
	if err != nil {
		return EvaluationResponse{}, err
	}

	return e.projectResponse(req.ScenarioID, outcome), nil
}

func (e *CortexEngine) projectResponse(scenarioID string, outcome *EvaluationOutcome) EvaluationResponse {
	routeTier := routeTierFromVerdict(outcome.FinalVerdict)
	fields := fieldViewsFromOutcome(outcome)
	qViews := questionViewsFromOutcome(outcome)

	modeStr := "CALIBRATED_JEV_SIMULATION"
	if outcome.Mode == "live_api" {
		modeStr = "LIVE_TYPESAFE_API"
	}

	return EvaluationResponse{
		Mode:           modeStr,
		FallbackUsed:   outcome.FallbackUsed,
		LiveError:      outcome.LiveError,
		ScenarioID:     scenarioID,
		Pipeline:       outcome.Pipeline,
		RequestID:      outcome.RequestID,
		FinalRouteTier: routeTier,
		FinalDecision:  outcome.VerdictSummary,
		SelectedSkill:  outcome.SelectedSkill,
		FailedFields:   outcome.FailedFields,
		CompositeScore: round3(outcome.CompositeTrust * 100.0),
		CompositeRisk:  round3(outcome.CompositeRiskScore * 100.0),
		LatencyMs:      float64(outcome.Economics.WallClockLatencyMs),
		InputTokens:    outcome.InputTokens,
		Cascade: CascadeEconomicsView{
			NaiveFrontierCostUSD:   outcome.Economics.UnroutedFrontierCostUSD,
			NaiveFrontierLatencyMs: float64(outcome.Economics.UnroutedLatencyMs),
			LegacyRouterCostUSD:    outcome.Economics.LegacyRouterCostUSD,
			LegacyRouterLatencyMs:  float64(outcome.Economics.LegacyRouterLatencyMs),
			AegisControlCostUSD:    outcome.Economics.TypeSafeCostUSD,
			AegisControlLatencyMs:  float64(outcome.Economics.WallClockLatencyMs),
			AegisBlendedCostUSD:    outcome.Economics.ExecutedTotalCostUSD,
			AegisTotalLatencyMs:    float64(outcome.Economics.WallClockLatencyMs),
			ReferenceJevP50Ms:      float64(outcome.Economics.ReferenceJevP50Ms),
			CostSavingsPercent:     outcome.Economics.CostSavingsPercent,
		},
		FieldVerifications: fields,
		Questions:          qViews,
		Flywheel:           e.GetFlywheel(),
		History:            e.historyView(8),
	}
}

func routeTierFromVerdict(verdict string) string {
	switch verdict {
	case "BLOCK_GUARDRAIL":
		return "TIER_0_BLOCK"
	case "AUTO_EXECUTE_TIER0":
		return "TIER_0_AUTO_EXEC"
	case "ESCALATE_REASONING_TIER2":
		return "TIER_2_SURGICAL_FIELD_REPAIR"
	default:
		return "TIER_1_VERIFIED_FASTPATH"
	}
}

func fieldViewsFromOutcome(outcome *EvaluationOutcome) []FieldVerificationView {
	failed := make(map[string]bool, len(outcome.FailedFields))
	for _, name := range outcome.FailedFields {
		failed[name] = true
	}
	out := make([]FieldVerificationView, 0, 4)
	for _, it := range outcome.Items {
		if !isFieldQuestion(it.Key) {
			continue
		}
		name := it.FieldName
		if name == "" {
			name = it.Key
		}
		ok := !failed[name] && !failed[it.Key] && it.GateAction == "pass"
		action := "Verified by field noul — locked"
		if !ok {
			action = "Failed field noul — surgical repair this field only"
		}
		out = append(out, FieldVerificationView{
			FieldName:  name,
			QuestionID: it.Key,
			YesProb:    it.NumericValue,
			NoProb:     round3(1.0 - it.NumericValue),
			Confidence: it.Confidence,
			Verified:   ok,
			Action:     action,
		})
	}
	return out
}

func questionViewsFromOutcome(outcome *EvaluationOutcome) []QuestionView {
	qViews := make([]QuestionView, 0, len(outcome.Items))
	for _, it := range outcome.Items {
		label := it.SelectedLabel
		if label == "" {
			if it.NumericValue >= 0.5 {
				label = "YES"
			} else {
				label = "NO"
			}
		}
		qViews = append(qViews, QuestionView{
			ID:             it.Key,
			Stage:          it.Category,
			Type:           strings.ToUpper(it.Primitive),
			Prompt:         it.Instructions,
			SelectedChoice: label,
			TopProbability: it.NumericValue,
			Confidence:     it.Confidence,
			AutoExecutable: it.GateAction == "act" || it.GateAction == "pass",
			RouteReason:    "Gate: " + strings.ToUpper(it.GateAction),
			Probabilities:  it.Probabilities,
		})
	}
	return qViews
}
