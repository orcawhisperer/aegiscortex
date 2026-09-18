package aegiscortex

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
)

// BacktestRequest is POST /api/backtest.
type BacktestRequest struct {
	JSONL         string              `json:"jsonl"`
	Turns         []EvaluationRequest `json:"turns"`
	Synthetic     int                 `json:"synthetic"`
	Thresholds    *PipelineThresholds `json:"thresholds,omitempty"`
	MonthlyVolume float64             `json:"monthly_volume"`
}

// BacktestReport is the executive FinOps ledger.
type BacktestReport struct {
	Turns              int                `json:"turns"`
	TierCounts         map[string]int     `json:"tier_counts"`
	TierRates          map[string]float64 `json:"tier_rates"`
	MeanAegisUSD       float64            `json:"mean_aegis_usd"`
	MeanBaselineUSD    float64            `json:"mean_baseline_usd"`
	SavingsPercent     float64            `json:"savings_percent"`
	MonthlyAegisUSD    float64            `json:"monthly_aegis_usd"`
	MonthlyBaselineUSD float64            `json:"monthly_baseline_usd"`
	MonthlySavingsUSD  float64            `json:"monthly_savings_usd"`
	MonthlyVolume      float64            `json:"monthly_volume"`
	Confusion          map[string]int     `json:"confusion"`
	Labeled            int                `json:"labeled"`
	MatchRate          float64            `json:"match_rate"`
}

func parseJSONLTurns(raw string) []EvaluationRequest {
	var out []EvaluationRequest
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req EvaluationRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			var wrap map[string]any
			if json.Unmarshal([]byte(line), &wrap) != nil {
				continue
			}
			req.Context = wrap
		}
		out = append(out, req)
	}
	return out
}

func syntheticTurns(n int) []EvaluationRequest {
	presets := DefaultPresets()
	if n <= 0 {
		n = 50
	}
	out := make([]EvaluationRequest, 0, n)
	for i := 0; i < n; i++ {
		p := presets[i%len(presets)]
		out = append(out, EvaluationRequest{
			ScenarioID:   p.ID,
			Context:      p.State,
			ExpectedTier: presetWantTier[p.ID],
			ReadOnly:     true,
		})
	}
	return out
}

func (e *CortexEngine) RunBacktest(ctx context.Context, req BacktestRequest) (BacktestReport, error) {
	turns := append([]EvaluationRequest{}, req.Turns...)
	if raw := strings.TrimSpace(req.JSONL); raw != "" {
		turns = append(turns, parseJSONLTurns(raw)...)
	}
	if req.Synthetic > 0 || len(turns) == 0 {
		n := req.Synthetic
		if n <= 0 {
			n = 50
		}
		turns = append(turns, syntheticTurns(n)...)
	}
	vol := req.MonthlyVolume
	if vol <= 0 {
		vol = 10_000_000
	}

	counts := map[string]int{}
	confusion := map[string]int{}
	sumAegis := 0.0
	sumBase := 0.0
	labeled := 0
	matches := 0

	for _, turn := range turns {
		turn.ReadOnly = true
		if req.Thresholds != nil {
			turn.Thresholds = req.Thresholds
		}
		res, err := e.EvaluateRequest(ctx, turn)
		if err != nil {
			continue
		}
		counts[res.FinalRouteTier]++
		sumAegis += res.Cascade.AegisBlendedCostUSD
		sumBase += res.Cascade.NaiveFrontierCostUSD
		if want := wantTierFor(turn.ScenarioID, turn.ExpectedTier); want != "" {
			labeled++
			key := want + "→" + res.FinalRouteTier
			confusion[key]++
			if want == res.FinalRouteTier {
				matches++
			}
		}
	}

	n := len(turns)
	if n == 0 {
		return BacktestReport{MonthlyVolume: vol}, nil
	}
	rates := map[string]float64{}
	for k, c := range counts {
		rates[k] = round3(float64(c) / float64(n))
	}
	meanA := sumAegis / float64(n)
	meanB := sumBase / float64(n)
	sav := 0.0
	if meanB > 0 {
		sav = ((meanB - meanA) / meanB) * 100
	}
	match := 0.0
	if labeled > 0 {
		match = float64(matches) / float64(labeled)
	}
	return BacktestReport{
		Turns:              n,
		TierCounts:         counts,
		TierRates:          rates,
		MeanAegisUSD:       meanA,
		MeanBaselineUSD:    meanB,
		SavingsPercent:     round3(sav),
		MonthlyAegisUSD:    meanA * vol,
		MonthlyBaselineUSD: meanB * vol,
		MonthlySavingsUSD:  (meanB - meanA) * vol,
		MonthlyVolume:      vol,
		Confusion:          confusion,
		Labeled:            labeled,
		MatchRate:          round3(match),
	}, nil
}
