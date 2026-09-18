package aegiscortex

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/orcawhisperer/typesafe-sdk-go"
)

const (
	typesafeInputUSDPerMTok = 0.042
	modeledMiniUSD          = 0.00085
	modeledFrontierUSD      = 0.02850
	modeledUnroutedUSD      = 0.03840
	modeledUnroutedLatency  = int64(11450)
	referenceJevP50Ms       = int64(114)
	historyLimit            = 24
)

// GateThresholds holds live-tunable confidence gates.
type GateThresholds struct {
	GuardrailBlockProb  float64 `json:"guardrail_block_prob"`
	ChoiceActConfidence float64 `json:"choice_act_confidence"`
	ReviewMinConfidence float64 `json:"review_min_confidence"`
	CascadeVerifyMin    float64 `json:"cascade_verify_min"`
	CompositePassScore  float64 `json:"composite_pass_score"`
}

// DefaultGateThresholds returns the calibrated defaults.
func DefaultGateThresholds() GateThresholds {
	return GateThresholds{
		GuardrailBlockProb:  0.65,
		ChoiceActConfidence: 0.80,
		ReviewMinConfidence: 0.50,
		CascadeVerifyMin:    0.75,
		CompositePassScore:  68.0,
	}
}

// QuestionEvaluationItem is one atomic decision in the fan-out matrix.
type QuestionEvaluationItem struct {
	Key           string             `json:"key"`
	Primitive     string             `json:"primitive"`
	Category      string             `json:"category"`
	Instructions  string             `json:"instructions"`
	FieldName     string             `json:"field_name,omitempty"`
	SelectedLabel string             `json:"selected_label,omitempty"`
	NumericValue  float64            `json:"numeric_value"`
	RawScore      float64            `json:"raw_score,omitempty"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	GateAction    string             `json:"gate_action"`
}

// CascadeComparison is modeled unit economics versus unrouted frontier+judge.
type CascadeComparison struct {
	WallClockLatencyMs      int64   `json:"wall_clock_latency_ms"`
	ReferenceJevP50Ms       int64   `json:"reference_jev_p50_ms"`
	TypeSafeCostUSD         float64 `json:"typesafe_cost_usd"`
	ExecutedTier            string  `json:"executed_tier"`
	ExecutedTotalCostUSD    float64 `json:"executed_total_cost_usd"`
	UnroutedFrontierCostUSD float64 `json:"unrouted_frontier_cost_usd"`
	UnroutedLatencyMs       int64   `json:"unrouted_latency_ms"`
	CostSavingsPercent      float64 `json:"cost_savings_percent"`
	SpeedupMultiplier       float64 `json:"speedup_multiplier"`
	LegacyRouterCostUSD     float64 `json:"legacy_router_cost_usd"`
	LegacyRouterLatencyMs   int64   `json:"legacy_router_latency_ms"`
}

// EvaluationOutcome is the full result of one fan-out evaluation.
type EvaluationOutcome struct {
	Timestamp          string                   `json:"timestamp"`
	Mode               string                   `json:"mode"`
	FallbackUsed       bool                     `json:"fallback_used"`
	LiveError          string                   `json:"live_error,omitempty"`
	Model              string                   `json:"model"`
	RequestID          string                   `json:"request_id"`
	Pipeline           PipelineID               `json:"pipeline"`
	FinalVerdict       string                   `json:"final_verdict"`
	VerdictSummary     string                   `json:"verdict_summary"`
	SelectedSkill      string                   `json:"selected_skill"`
	FailedFields       []string                 `json:"failed_fields"`
	CompositeRiskScore float64                  `json:"composite_risk_score"`
	CompositeTrust     float64                  `json:"composite_trust"`
	Items              []QuestionEvaluationItem `json:"items"`
	Economics          CascadeComparison        `json:"economics"`
	InputTokens        int                      `json:"input_tokens"`
	OutputTokens       int                      `json:"output_tokens"`
}

// FlywheelMetrics is cumulative telemetry plus recommended gates.
type FlywheelMetrics struct {
	TotalRequests          int     `json:"total_requests"`
	TotalAtomicQuestions   int     `json:"total_atomic_questions"`
	GuardrailsBlocked      int     `json:"guardrails_blocked"`
	HallucinationsCaught   int     `json:"hallucinations_caught"`
	FastPathApproved       int     `json:"fast_path_approved"`
	AutoExecuted           int     `json:"auto_executed"`
	ReasoningEscalations   int     `json:"reasoning_escalations"`
	TotalAegisCostUSD      float64 `json:"total_aegis_cost_usd"`
	TotalBaselineCostUSD   float64 `json:"total_baseline_cost_usd"`
	ExpectedCalibrationECE float64 `json:"expected_calibration_ece"`
	RecommendedActGate     float64 `json:"recommended_act_gate"`
	RecommendedBlockProb   float64 `json:"recommended_block_prob"`
	RecommendedVerifyMin   float64 `json:"recommended_verify_min"`
	RecommendedComposite   float64 `json:"recommended_composite"`
	sumAbsError            float64
	noulObservations       int
}

// CortexEngine owns the TypeSafe client config, thresholds, history, and flywheel.
type CortexEngine struct {
	mu         sync.RWMutex
	apiKey     string
	baseURL    string
	client     *typesafe.Client
	clientKey  string
	thresholds GateThresholds
	history    []EvaluationOutcome
	metrics    FlywheelMetrics
	labels     []LabeledTurn
	lastSolve  SolvedGates
	limiter    *ipLimiter
	simScorer  SpeculativeScorer
	liveScorer SpeculativeScorer
}

// NewCortexEngine initializes the engine from AEGIS_ENGINE_KEY / TYPESAFE_API_KEY.
func NewCortexEngine() *CortexEngine {
	envKey := EngineKeyFromEnv()
	envURL := strings.TrimSpace(os.Getenv(typesafe.BaseURLEnv))
	if envURL == "" {
		envURL = typesafe.DefaultBaseURL
	}
	e := &CortexEngine{
		apiKey:     envKey,
		baseURL:    envURL,
		thresholds: DefaultGateThresholds(),
		limiter:    newIPLimiter(),
		simScorer:  simulatorScorer{},
		metrics: FlywheelMetrics{
			ExpectedCalibrationECE: 0.018,
			RecommendedActGate:     0.80,
			RecommendedBlockProb:   0.65,
			RecommendedVerifyMin:   0.75,
			RecommendedComposite:   68.0,
		},
	}
	e.liveScorer = typesafeScorer{engine: e}
	return e
}

// SetAPIKey updates the in-memory TypeSafe API key (never written to disk).
func (e *CortexEngine) SetAPIKey(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key = strings.TrimSpace(key)
	if key == e.apiKey {
		return
	}
	e.apiKey = key
	if e.client != nil {
		e.client.Close()
		e.client = nil
		e.clientKey = ""
	}
}

func (e *CortexEngine) acquireLiveClient() (*typesafe.Client, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.apiKey == "" {
		return nil, nil
	}
	if e.client != nil && e.clientKey == e.apiKey {
		return e.client, nil
	}
	if e.client != nil {
		e.client.Close()
		e.client = nil
	}
	client, err := typesafe.NewClient(
		typesafe.WithAPIKey(e.apiKey),
		typesafe.WithBaseURL(e.baseURL),
		typesafe.WithTimeout(20*time.Second),
		typesafe.WithDefaultModel(typesafe.ModelJev1_13_0),
	)
	if err != nil {
		return nil, err
	}
	e.client = client
	e.clientKey = e.apiKey
	return client, nil
}

// HasLiveKey reports whether a non-empty API key is configured in memory.
func (e *CortexEngine) HasLiveKey() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.apiKey != ""
}

// UpdateThresholds updates gates, ignoring out-of-range values.
func (e *CortexEngine) UpdateThresholds(t GateThresholds) GateThresholds {
	e.mu.Lock()
	defer e.mu.Unlock()
	if t.GuardrailBlockProb > 0 && t.GuardrailBlockProb <= 1 {
		e.thresholds.GuardrailBlockProb = t.GuardrailBlockProb
	}
	if t.ChoiceActConfidence > 0 && t.ChoiceActConfidence <= 1 {
		e.thresholds.ChoiceActConfidence = t.ChoiceActConfidence
	}
	if t.ReviewMinConfidence >= 0 && t.ReviewMinConfidence <= 1 {
		e.thresholds.ReviewMinConfidence = t.ReviewMinConfidence
	}
	if t.CascadeVerifyMin > 0 && t.CascadeVerifyMin <= 1 {
		e.thresholds.CascadeVerifyMin = t.CascadeVerifyMin
	}
	if t.CompositePassScore >= 30 && t.CompositePassScore <= 100 {
		e.thresholds.CompositePassScore = t.CompositePassScore
	}
	return e.thresholds
}

func (e *CortexEngine) snapshotThresholds() GateThresholds {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.thresholds
}

func (e *CortexEngine) snapshotHistory() []EvaluationOutcome {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]EvaluationOutcome, len(e.history))
	copy(out, e.history)
	return out
}

func (e *CortexEngine) snapshotMetrics() FlywheelMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.metrics
}

func (e *CortexEngine) snapshotSolve() SolvedGates {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastSolve
}

func (e *CortexEngine) AllowEvaluate(ip string) bool {
	if !e.HasLiveKey() {
		return true
	}
	if e.limiter == nil {
		return true
	}
	return e.limiter.allow(ip, 20, time.Minute)
}

type evaluateOpts struct {
	Pipeline     PipelineID
	State        any
	PreferLive   bool
	Schema       map[string]any
	Thresholds   *GateThresholds
	ReadOnly     bool
	ScenarioID   string
	ExpectedTier string
	ExtraFields  []CompiledField
}

// Evaluate runs the 11-question fan-out. Live API is used when a key is present;
// failures are recorded and the calibrated simulator is used instead.
func (e *CortexEngine) Evaluate(ctx context.Context, pipeline PipelineID, state any, preferLive bool) (*EvaluationOutcome, error) {
	return e.evaluate(ctx, evaluateOpts{Pipeline: pipeline, State: state, PreferLive: preferLive})
}

func (e *CortexEngine) evaluate(ctx context.Context, opts evaluateOpts) (*EvaluationOutcome, error) {
	e.mu.RLock()
	thresh := e.thresholds
	e.mu.RUnlock()
	if opts.Thresholds != nil {
		thresh = *opts.Thresholds
	}

	questions, specs := BuildQuestions(opts.Pipeline, opts.Schema, opts.ExtraFields)
	start := time.Now()

	var sdkResp *typesafe.SystemOneResponse
	mode := "calibrated_rlcd_sim"
	fallback := false
	liveErr := ""

	if opts.PreferLive && e.liveScorer != nil {
		liveResp, callErr := e.liveScorer.Score(ctx, opts.Pipeline, opts.State, questions, specs)
		if callErr != nil {
			fallback = true
			liveErr = sanitizeLiveError(callErr)
		} else if liveResp != nil {
			sdkResp = liveResp
			mode = "live_api"
		}
	}

	if sdkResp == nil {
		sdkResp, _ = e.simScorer.Score(ctx, opts.Pipeline, opts.State, questions, specs)
	}

	elapsedMs := time.Since(start).Milliseconds()
	if elapsedMs < 1 {
		elapsedMs = 1
	}

	items, failedFields, guardrailTriggered := buildItems(sdkResp, specs, thresh)

	riskComp, riskErr := typesafe.ComputeCompositeScore(sdkResp, []typesafe.ScoreDimension{
		{Name: qJailbreak, Weight: 0.35},
		{Name: qRAGInjection, Weight: 0.35},
		{Name: qCredentialPII, Weight: 0.15},
		{Name: qHarmSeverity, Weight: 0.15, MaxScore: 2.0},
	})
	trustComp, trustErr := typesafe.ComputeCompositeScore(sdkResp, fieldTrustDimensions(specs))

	compositeRisk := 0.0
	if riskErr == nil && riskComp != nil {
		compositeRisk = riskComp.WeightedScore
	}
	compositeTrust := 0.0
	if trustErr == nil && trustComp != nil {
		compositeTrust = trustComp.WeightedScore
	}

	citationChoice := sdkResp.Choices[qCitation]
	execTierChoice := sdkResp.Choices[qExecTier]
	skillChoice := sdkResp.Choices[qAgentSkill]

	inTokens := sdkResp.Usage.InputTokensValue()
	if inTokens <= 0 {
		inTokens = 980
	}
	tsCost := (float64(inTokens) / 1_000_000.0) * typesafeInputUSDPerMTok

	verdict, summary, executedTier, executedTotalCost := decideRoute(
		thresh,
		guardrailTriggered,
		compositeRisk,
		compositeTrust,
		failedFields,
		citationChoice,
		execTierChoice,
		skillChoice,
		tsCost,
	)

	savingsPct := ((modeledUnroutedUSD - executedTotalCost) / modeledUnroutedUSD) * 100.0
	if savingsPct < 0 {
		savingsPct = 0
	}

	outcome := EvaluationOutcome{
		Timestamp:          time.Now().UTC().Format("15:04:05.000"),
		Mode:               mode,
		FallbackUsed:       fallback,
		LiveError:          liveErr,
		Model:              sdkResp.Model,
		RequestID:          sdkResp.RequestID,
		Pipeline:           opts.Pipeline,
		FinalVerdict:       verdict,
		VerdictSummary:     summary,
		SelectedSkill:      skillChoice.Choice,
		FailedFields:       failedFields,
		CompositeRiskScore: round3(compositeRisk),
		CompositeTrust:     round3(compositeTrust),
		Items:              items,
		InputTokens:        inTokens,
		OutputTokens:       sdkResp.Usage.OutputTokensValue(),
		Economics: CascadeComparison{
			WallClockLatencyMs:      elapsedMs,
			ReferenceJevP50Ms:       referenceJevP50Ms,
			TypeSafeCostUSD:         tsCost,
			ExecutedTier:            executedTier,
			ExecutedTotalCostUSD:    executedTotalCost,
			UnroutedFrontierCostUSD: modeledUnroutedUSD,
			UnroutedLatencyMs:       modeledUnroutedLatency,
			CostSavingsPercent:      round3(savingsPct),
			SpeedupMultiplier:       round3(float64(modeledUnroutedLatency) / float64(maxInt64(referenceJevP50Ms, 1))),
			LegacyRouterCostUSD:     0.0042,
			LegacyRouterLatencyMs:   620,
		},
	}

	if !opts.ReadOnly {
		e.recordOutcome(outcome, sdkResp)
		if want := wantTierFor(opts.ScenarioID, opts.ExpectedTier); want != "" {
			e.rememberTurn(labelFromOutcome(opts.ScenarioID, want, &outcome))
			_, _ = e.solveAndRecommend(0.001)
		}
	}
	return &outcome, nil
}

func fieldTrustDimensions(specs []BoundQuestionSpec) []typesafe.ScoreDimension {
	keys := make([]string, 0, 4)
	for _, spec := range specs {
		if spec.Primitive == "noul" && isFieldQuestion(spec.Key) {
			keys = append(keys, spec.Key)
		}
	}
	if len(keys) == 0 {
		keys = []string{qFieldVendor, qFieldAmount, qFieldDate, qFieldClaim}
	}
	w := 1.0 / float64(len(keys))
	out := make([]typesafe.ScoreDimension, 0, len(keys))
	for _, k := range keys {
		out = append(out, typesafe.ScoreDimension{Name: k, Weight: w})
	}
	return out
}

func buildItems(sdkResp *typesafe.SystemOneResponse, specs []BoundQuestionSpec, thresh GateThresholds) ([]QuestionEvaluationItem, []string, bool) {
	items := make([]QuestionEvaluationItem, 0, len(specs))
	failedFields := make([]string, 0, 4)
	guardrailTriggered := false

	for _, spec := range specs {
		item := QuestionEvaluationItem{
			Key:          spec.Key,
			Primitive:    spec.Primitive,
			Category:     spec.Category,
			Instructions: spec.Instructions,
			FieldName:    spec.FieldName,
		}
		switch spec.Primitive {
		case "noul":
			nAns := sdkResp.Nouls[spec.Key]
			item.NumericValue = round3(nAns.Noul)
			item.Confidence = round3(math.Abs(nAns.Noul-0.5) * 2.0)
			if isGuardrailNoul(spec.Key) {
				if nAns.Noul >= thresh.GuardrailBlockProb {
					item.GateAction = "block"
					guardrailTriggered = true
				} else if nAns.Noul >= thresh.GuardrailBlockProb*0.65 {
					item.GateAction = "review"
				} else {
					item.GateAction = "pass"
				}
			} else if isFieldQuestion(spec.Key) {
				if nAns.Noul >= thresh.CascadeVerifyMin {
					item.GateAction = "pass"
				} else {
					item.GateAction = "review"
					name := spec.FieldName
					if name == "" {
						name = spec.Key
					}
					failedFields = append(failedFields, name)
				}
			} else {
				dec := typesafe.RouteNoul(nAns, thresh.CascadeVerifyMin, 0.30)
				item.GateAction = string(dec.Action)
			}
		case "choice":
			cAns := sdkResp.Choices[spec.Key]
			item.SelectedLabel = cAns.Choice
			item.Confidence = round3(cAns.Confidence)
			item.NumericValue = round3(cAns.Probabilities[cAns.Choice])
			item.Probabilities = cAns.Probabilities
			dec := typesafe.RouteChoice(cAns, thresh.ChoiceActConfidence, thresh.ReviewMinConfidence)
			item.GateAction = string(dec.Action)
		case "score":
			sAns := sdkResp.Scores[spec.Key]
			item.RawScore = round3(sAns.Score)
			item.NumericValue = round3(sAns.Score / 2.0)
			item.Confidence = round3(sAns.Confidence)
			item.Probabilities = sAns.Probabilities
			item.SelectedLabel = scoreBandLabel(sAns.Score)
			dec := typesafe.RouteScore(sAns, thresh.ChoiceActConfidence, thresh.ReviewMinConfidence)
			item.GateAction = string(dec.Action)
		}
		items = append(items, item)
	}
	return items, failedFields, guardrailTriggered
}

func decideRoute(
	thresh GateThresholds,
	guardrailTriggered bool,
	compositeRisk float64,
	compositeTrust float64,
	failedFields []string,
	citation typesafe.ChoiceResponse[string],
	execTier typesafe.ChoiceResponse[string],
	skill typesafe.ChoiceResponse[string],
	tsCost float64,
) (verdict, summary, executedTier string, executedTotalCost float64) {
	trustScore := compositeTrust * 100.0

	switch {
	case guardrailTriggered || compositeRisk >= thresh.GuardrailBlockProb:
		verdict = "BLOCK_GUARDRAIL"
		executedTier = "Tier 0: Deterministic firewall block"
		executedTotalCost = tsCost
		summary = fmt.Sprintf(
			"Blocked before generation. Guardrail noul or composite risk (%.2f) met τ_sec=%.2f. Modeled downstream LLM cost is $0.00.",
			compositeRisk, thresh.GuardrailBlockProb,
		)

	case len(failedFields) > 0 || citation.Choice == "contradicted" || citation.Choice == "extrapolated":
		verdict = "ESCALATE_REASONING_TIER2"
		executedTier = "Tier 2: Surgical field repair / frontier reasoning"
		executedTotalCost = tsCost + modeledMiniUSD + modeledFrontierUSD
		fieldNote := "citation=" + citation.Choice
		if len(failedFields) > 0 {
			fieldNote = "failed fields: " + strings.Join(failedFields, ", ")
		}
		summary = fmt.Sprintf(
			"Per-field verifier refused to lock every extraction (%s). Escalating only the uncertain work to frontier reasoning. Trust=%.1f/100.",
			fieldNote, trustScore,
		)

	case trustScore < thresh.CompositePassScore:
		verdict = "ESCALATE_REASONING_TIER2"
		executedTier = "Tier 2: Uncertainty-gated escalation"
		executedTotalCost = tsCost + modeledFrontierUSD
		summary = fmt.Sprintf(
			"Composite quality (%.1f/100) is below the pass gate (%.0f). Routing to Tier-2 review.",
			trustScore, thresh.CompositePassScore,
		)

	case execTier.Choice == "tier0_deterministic" && execTier.Confidence >= thresh.ChoiceActConfidence && skill.Choice != "none_needed" && skill.Choice != "incident_pager_alert":
		verdict = "AUTO_EXECUTE_TIER0"
		executedTier = "Tier 0: Autonomous deterministic skill (" + skill.Choice + ")"
		executedTotalCost = tsCost
		summary = fmt.Sprintf(
			"High-confidence policy match (execution_tier conf=%.2f ≥ τ_route=%.2f). Modeled action: `%s` with $0.00 generation cost. No webhook is fired from this studio.",
			execTier.Confidence, thresh.ChoiceActConfidence, skill.Choice,
		)

	default:
		verdict = "VERIFIED_FASTPATH_TIER1"
		executedTier = "Tier 1: Fast mini + verified fan-out"
		executedTotalCost = tsCost + modeledMiniUSD
		summary = fmt.Sprintf(
			"All field nouls locked at τ_field=%.2f and citation is %s (trust=%.1f/100 ≥ %.0f). Serve the mini draft; skip frontier reasoning.",
			thresh.CascadeVerifyMin, citation.Choice, trustScore, thresh.CompositePassScore,
		)
	}
	return verdict, summary, executedTier, executedTotalCost
}

func (e *CortexEngine) recordOutcome(o EvaluationOutcome, resp *typesafe.SystemOneResponse) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metrics.TotalRequests++
	e.metrics.TotalAtomicQuestions += len(o.Items)
	e.metrics.TotalAegisCostUSD += o.Economics.ExecutedTotalCostUSD
	e.metrics.TotalBaselineCostUSD += o.Economics.UnroutedFrontierCostUSD

	switch o.FinalVerdict {
	case "BLOCK_GUARDRAIL":
		e.metrics.GuardrailsBlocked++
	case "AUTO_EXECUTE_TIER0":
		e.metrics.AutoExecuted++
		e.metrics.FastPathApproved++
	case "VERIFIED_FASTPATH_TIER1":
		e.metrics.FastPathApproved++
	case "ESCALATE_REASONING_TIER2":
		e.metrics.ReasoningEscalations++
		if len(o.FailedFields) > 0 || o.CompositeTrust < 0.75 {
			e.metrics.HallucinationsCaught++
		}
	}

	if resp != nil {
		for _, noul := range resp.Nouls {
			label := 0.0
			if noul.Noul >= 0.5 {
				label = 1
			}
			e.metrics.sumAbsError += math.Abs(noul.Noul - label)
			e.metrics.noulObservations++
		}
		if e.metrics.noulObservations > 0 {
			e.metrics.ExpectedCalibrationECE = round3(e.metrics.sumAbsError / float64(e.metrics.noulObservations) / 2.0)
		}
	}

	// Heuristic flywheel must not clobber a solver-produced τ vector.
	if e.lastSolve.Samples == 0 {
		if e.metrics.GuardrailsBlocked > 0 {
			e.metrics.RecommendedBlockProb = round3(clamp(e.thresholds.GuardrailBlockProb*0.98+0.02, 0.50, 0.92))
		}
		if e.metrics.FastPathApproved > 0 {
			e.metrics.RecommendedActGate = round3(clamp(e.thresholds.ChoiceActConfidence, 0.70, 0.95))
			e.metrics.RecommendedVerifyMin = round3(clamp(e.thresholds.CascadeVerifyMin, 0.70, 0.95))
			e.metrics.RecommendedComposite = round3(clamp(e.thresholds.CompositePassScore, 50, 90))
		}
	}
	e.history = append([]EvaluationOutcome{o}, e.history...)
	if len(e.history) > historyLimit {
		e.history = e.history[:historyLimit]
	}
}

func scoreBandLabel(score float64) string {
	switch {
	case score < 0.67:
		return "0: Negligible"
	case score < 1.34:
		return "1: Moderate"
	default:
		return "2: Severe"
	}
}

func sanitizeLiveError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 280 {
		msg = msg[:280] + "…"
	}
	return msg
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
