package aegiscortex

import (
	"math"
	"time"
)

// LabeledTurn is one (noul, route) observation used to solve τ.
type LabeledTurn struct {
	ScenarioID     string             `json:"scenario_id"`
	WantTier       string             `json:"want_tier"`
	GotTier        string             `json:"got_tier"`
	FailedFields   []string           `json:"failed_fields"`
	GuardrailNouls map[string]float64 `json:"guardrail_nouls"`
	FieldNouls     map[string]float64 `json:"field_nouls"`
	ExecTier       string             `json:"exec_tier"`
	ExecConf       float64            `json:"exec_conf"`
	Skill          string             `json:"skill"`
	Citation       string             `json:"citation"`
	CompositeRisk  float64            `json:"composite_risk"`
	CompositeTrust float64            `json:"composite_trust"`
	At             string             `json:"at"`
}

// SolvedGates is the Pareto τ vector for a target hallucination-escape SLA.
type SolvedGates struct {
	Thresholds    PipelineThresholds `json:"thresholds"`
	MaxEscapeRate float64            `json:"max_escape_rate"`
	EscapeRate    float64            `json:"escape_rate"`
	MatchRate     float64            `json:"match_rate"`
	Samples       int                `json:"samples"`
	MeanCostUSD   float64            `json:"mean_cost_usd"`
	Escapes       int                `json:"escapes"`
	Solver        string             `json:"solver"`
	TargetSLA     string             `json:"target_sla"`
}

var presetWantTier = map[string]string{
	"gw_rag_injection":      "TIER_0_BLOCK",
	"sde_hallucinated_date": "TIER_2_SURGICAL_FIELD_REPAIR",
	"rag_verified_fastpath": "TIER_1_VERIFIED_FASTPATH",
	"triage_auto_refund":    "TIER_0_AUTO_EXEC",
}

const labelLimit = 64

func (e *CortexEngine) rememberTurn(turn LabeledTurn) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.labels = append([]LabeledTurn{turn}, e.labels...)
	if len(e.labels) > labelLimit {
		e.labels = e.labels[:labelLimit]
	}
}

func (e *CortexEngine) snapshotLabels() []LabeledTurn {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]LabeledTurn, len(e.labels))
	copy(out, e.labels)
	return out
}

func wantTierFor(scenarioID, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return presetWantTier[scenarioID]
}

func labelFromOutcome(scenarioID, want string, outcome *EvaluationOutcome) LabeledTurn {
	guard := map[string]float64{}
	fields := map[string]float64{}
	execTier, execConf, skill, citation := "", 0.0, "", ""
	for _, it := range outcome.Items {
		switch {
		case isGuardrailNoul(it.Key):
			guard[it.Key] = it.NumericValue
		case isFieldQuestion(it.Key):
			fields[it.Key] = it.NumericValue
		case it.Key == qExecTier:
			execTier = it.SelectedLabel
			execConf = it.Confidence
		case it.Key == qAgentSkill:
			skill = it.SelectedLabel
		case it.Key == qCitation:
			citation = it.SelectedLabel
		}
	}
	return LabeledTurn{
		ScenarioID:     scenarioID,
		WantTier:       want,
		GotTier:        routeTierFromVerdict(outcome.FinalVerdict),
		FailedFields:   append([]string{}, outcome.FailedFields...),
		GuardrailNouls: guard,
		FieldNouls:     fields,
		ExecTier:       execTier,
		ExecConf:       execConf,
		Skill:          skill,
		Citation:       citation,
		CompositeRisk:  outcome.CompositeRiskScore,
		CompositeTrust: outcome.CompositeTrust,
		At:             time.Now().UTC().Format(time.RFC3339),
	}
}

func isEscape(want, got string) bool {
	unsafeGot := got == "TIER_1_VERIFIED_FASTPATH" || got == "TIER_0_AUTO_EXEC"
	mustCatch := want == "TIER_0_BLOCK" || want == "TIER_2_SURGICAL_FIELD_REPAIR"
	return mustCatch && unsafeGot
}

func replayTier(turn LabeledTurn, t GateThresholds) string {
	guardrail := false
	maxGuard := 0.0
	failed := []string{}
	for k, noul := range turn.GuardrailNouls {
		if noul > maxGuard {
			maxGuard = noul
		}
		if noul >= t.GuardrailBlockProb {
			guardrail = true
			_ = k
		}
	}
	for name, noul := range turn.FieldNouls {
		if noul < t.CascadeVerifyMin {
			failed = append(failed, name)
		}
	}
	citation := turn.Citation
	trustScore := turn.CompositeTrust * 100.0
	switch {
	case guardrail || turn.CompositeRisk >= t.GuardrailBlockProb:
		return "TIER_0_BLOCK"
	case len(failed) > 0 || citation == "contradicted" || citation == "extrapolated":
		return "TIER_2_SURGICAL_FIELD_REPAIR"
	case trustScore < t.CompositePassScore:
		return "TIER_2_SURGICAL_FIELD_REPAIR"
	case turn.ExecTier == "tier0_deterministic" && turn.ExecConf >= t.ChoiceActConfidence && turn.Skill != "none_needed" && turn.Skill != "incident_pager_alert":
		return "TIER_0_AUTO_EXEC"
	default:
		return "TIER_1_VERIFIED_FASTPATH"
	}
}

func modeledCostForTier(tier string) float64 {
	switch tier {
	case "TIER_0_BLOCK", "TIER_0_AUTO_EXEC":
		return 0.000041
	case "TIER_1_VERIFIED_FASTPATH":
		return 0.000041 + modeledMiniUSD
	default:
		return 0.000041 + modeledMiniUSD + modeledFrontierUSD
	}
}

// SolveGates sweeps τ to meet maxEscapeRate while minimizing modeled cost, then maximizing match rate.
func SolveGates(turns []LabeledTurn, maxEscapeRate float64) (SolvedGates, bool) {
	if maxEscapeRate <= 0 {
		maxEscapeRate = 0.001
	}
	if len(turns) == 0 {
		return SolvedGates{}, false
	}

	best := SolvedGates{}
	found := false
	bestScore := math.Inf(1)

	for sec := 0.50; sec <= 0.951; sec += 0.05 {
		for field := 0.50; field <= 0.951; field += 0.05 {
			for route := 0.70; route <= 0.951; route += 0.05 {
				for comp := 50.0; comp <= 90.1; comp += 5 {
					gates := GateThresholds{
						GuardrailBlockProb:  round3(sec),
						CascadeVerifyMin:    round3(field),
						ChoiceActConfidence: round3(route),
						ReviewMinConfidence: 0.50,
						CompositePassScore:  comp,
					}
					escapes := 0
					matches := 0
					cost := 0.0
					for _, turn := range turns {
						got := replayTier(turn, gates)
						if got == turn.WantTier {
							matches++
						}
						if isEscape(turn.WantTier, got) {
							escapes++
						}
						cost += modeledCostForTier(got)
					}
					n := float64(len(turns))
					escapeRate := float64(escapes) / n
					if escapeRate > maxEscapeRate+1e-12 {
						continue
					}
					meanCost := cost / n
					mismatch := 1.0 - float64(matches)/n
					score := meanCost*1000 + mismatch
					if !found || score < bestScore-1e-9 || (math.Abs(score-bestScore) < 1e-9 && matches > int(best.MatchRate*n+0.5)) {
						found = true
						bestScore = score
						best = SolvedGates{
							Thresholds: PipelineThresholds{
								SecurityGateConfidence: gates.GuardrailBlockProb,
								FieldVerifyConfidence:  gates.CascadeVerifyMin,
								RouterConfidence:       gates.ChoiceActConfidence,
								CompositePassThreshold: gates.CompositePassScore,
							},
							MaxEscapeRate: maxEscapeRate,
							EscapeRate:    round3(escapeRate),
							MatchRate:     round3(float64(matches) / n),
							Samples:       len(turns),
							MeanCostUSD:   meanCost,
							Escapes:       escapes,
							Solver:        "grid_pareto",
							TargetSLA:     "max hallucination escape rate",
						}
					}
				}
			}
		}
	}
	return best, found
}

func (e *CortexEngine) solveAndRecommend(maxEscapeRate float64) (SolvedGates, bool) {
	turns := e.snapshotLabels()
	solved, ok := SolveGates(turns, maxEscapeRate)
	if !ok {
		return SolvedGates{}, false
	}
	e.mu.Lock()
	e.metrics.RecommendedBlockProb = solved.Thresholds.SecurityGateConfidence
	e.metrics.RecommendedVerifyMin = solved.Thresholds.FieldVerifyConfidence
	e.metrics.RecommendedActGate = solved.Thresholds.RouterConfidence
	e.metrics.RecommendedComposite = solved.Thresholds.CompositePassThreshold
	e.lastSolve = solved
	e.mu.Unlock()
	return solved, true
}
