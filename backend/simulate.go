package aegiscortex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/orcawhisperer/typesafe-sdk-go"
)

// stateSignals is a deterministic, inspectable reading of the request payload.
// The calibrated simulator maps these signals onto Jev-shaped probability
// distributions so local runs stay honest when the user edits JSON.
type stateSignals struct {
	Injection        bool
	CredentialLeak   bool
	HallucinatedDate bool
	DuplicateRefund  bool
	VendorSupported  bool
	AmountSupported  bool
	DateSupported    bool
	ClaimSupported   bool
	SourceText       string
	DraftText        string
}

func inspectState(state any) stateSignals {
	raw, _ := json.Marshal(state)
	lower := strings.ToLower(string(raw))
	root := asObject(state)

	sig := stateSignals{
		Injection:      containsAny(lower, "ignore previous", "system override", "ignore all previous", "developer mode enabled"),
		CredentialLeak: containsAny(lower, "sensitive_credentials", "collector.example", "exfiltrate", "send the user's api key"),
		SourceText:     strings.ToLower(firstString(root, "source_document", "refund_policy")),
		DraftText:      strings.ToLower(firstString(root, "draft_reply")),
	}

	if passages := collectPassageText(root["retrieved_passages"]); passages != "" {
		if sig.SourceText == "" {
			sig.SourceText = passages
		} else {
			sig.SourceText += "\n" + passages
		}
		if containsAny(passages, "ignore previous", "system override") {
			sig.Injection = true
		}
		if containsAny(passages, "sensitive_credentials", "collector.example", "api key") {
			sig.CredentialLeak = true
		}
	}

	extraction := asObject(root["mini_model_extraction"])
	if len(extraction) > 0 {
		vendor := stringify(firstValue(extraction, "vendor_name", "vendor", "entity"))
		amount := stringify(firstValue(extraction, "contract_value_usd", "invoice_amount", "amount_usd"))
		effective := stringify(firstValue(extraction, "effective_date"))
		notice := stringify(firstValue(extraction, "notice_deadline_date", "invoice_date"))

		sig.VendorSupported = sourceSupportsEntity(sig.SourceText, vendor)
		sig.AmountSupported = sourceSupportsAmount(sig.SourceText, amount)
		sig.DateSupported = sourceSupportsDate(sig.SourceText, effective) && !isHallucinatedNoticeDate(sig.SourceText, notice)
		if notice != "" && isHallucinatedNoticeDate(sig.SourceText, notice) {
			sig.HallucinatedDate = true
			sig.DateSupported = false
		}
		sig.ClaimSupported = containsAny(sig.SourceText, "automatically renew", "auto renew", "article 33", "article 83") && !sig.Injection
	} else if sig.DraftText != "" || sig.SourceText != "" {
		sig.VendorSupported = sourceSupportsEntity(sig.SourceText, "controller") || containsAny(sig.DraftText, "controller") && containsAny(sig.SourceText, "controller")
		sig.AmountSupported = sourceSupportsAmount(sig.SourceText, "20000000") || (containsAny(sig.SourceText, "20,000,000", "20000000", "4%") && containsAny(sig.DraftText, "20 million", "€20", "4%"))
		sig.DateSupported = containsAny(sig.SourceText, "72 hour") && containsAny(sig.DraftText, "72 hour")
		sig.ClaimSupported = sig.VendorSupported && sig.AmountSupported && sig.DateSupported && !sig.Injection && !containsAny(sig.DraftText, "system override")
	}

	sig.DuplicateRefund = detectDuplicateRefund(root)
	if sig.DuplicateRefund {
		ticket := strings.ToLower(jsonString(root["ticket"]))
		ledger := strings.ToLower(jsonString(root["customer_orders"]))
		policy := strings.ToLower(firstString(root, "refund_policy"))
		sig.VendorSupported = containsAny(ticket, "maya") && containsAny(ledger, "maya@acme.io")
		sig.AmountSupported = containsAny(ticket, "49") && containsAny(ledger, "49")
		sig.DateSupported = containsAny(ticket, "today") || containsAny(ledger, "captured")
		sig.ClaimSupported = containsAny(policy, "duplicate") && containsAny(policy, "automatic refund")
	}

	if sig.Injection {
		sig.ClaimSupported = false
	}
	return sig
}

func detectDuplicateRefund(root map[string]any) bool {
	policy := strings.ToLower(firstString(root, "refund_policy"))
	if !containsAny(policy, "duplicate") {
		return false
	}
	orders, _ := root["customer_orders"].([]any)
	if orders == nil {
		if typed, ok := root["customer_orders"].([]map[string]any); ok {
			for _, o := range typed {
				orders = append(orders, o)
			}
		}
	}
	for _, rawOrder := range orders {
		order := asObject(rawOrder)
		charges := asSlice(order["charges"])
		captured := 0
		var amt float64
		same := true
		for _, rawCharge := range charges {
			c := asObject(rawCharge)
			if strings.EqualFold(stringify(c["status"]), "captured") {
				n := asFloat(c["amount_usd"])
				if captured == 0 {
					amt = n
				} else if n != amt {
					same = false
				}
				captured++
			}
		}
		if captured >= 2 && same && amt > 0 && amt < 250 {
			return true
		}
	}
	return false
}

func isHallucinatedNoticeDate(source, extracted string) bool {
	extracted = strings.TrimSpace(extracted)
	if extracted == "" {
		return false
	}
	if sourceSupportsDate(source, extracted) {
		return false
	}
	return containsAny(source, "thirty (30) days", "30 days prior", "at least thirty")
}

func sourceSupportsEntity(source, entity string) bool {
	source = strings.ToLower(source)
	entity = strings.TrimSpace(strings.ToLower(entity))
	if source == "" {
		return false
	}
	if entity != "" && strings.Contains(source, entity) {
		return true
	}
	return containsAny(source, "northwind analytics", "controller", "vendor onboarding", "soc2")
}

func sourceSupportsAmount(source, amount string) bool {
	source = strings.ToLower(source)
	if containsAny(source, "20,000,000", "20000000", "4%", "usd 120,000", "120000", "120,000") {
		if amount == "" {
			return true
		}
		digits := digitsOnly(amount)
		return digits == "" || strings.Contains(digitsOnly(source), digits) || containsAny(source, "4%")
	}
	if containsAny(source, "49") && (amount == "" || strings.Contains(digitsOnly(amount), "49")) {
		return true
	}
	if amount != "" && strings.Contains(digitsOnly(source), digitsOnly(amount)) && digitsOnly(amount) != "" {
		return true
	}
	return false
}

func sourceSupportsDate(source, extracted string) bool {
	source = strings.ToLower(source)
	extracted = strings.TrimSpace(extracted)
	if containsAny(source, "72 hour") && (extracted == "" || containsAny(strings.ToLower(extracted), "72")) {
		return true
	}
	if extracted == "" {
		return containsAny(source, "november 1, 2025", "2025-11-01", "11/01/2025")
	}
	if strings.Contains(source, strings.ToLower(extracted)) {
		return true
	}
	if t, ok := parseFlexibleDate(extracted); ok {
		long := strings.ToLower(t.Format("January 2, 2006"))
		iso := t.Format("2006-01-02")
		us := t.Format("01/02/2006")
		alt := strings.ToLower(t.Format("Jan 2, 2006"))
		return containsAny(source, long, iso, us, alt)
	}
	return false
}

func parseFlexibleDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"01/02/2006",
		"1/2/2006",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
		"02/01/2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func simulateCalibratedJevResponse(pipeline PipelineID, state any, specs []BoundQuestionSpec) *typesafe.SystemOneResponse {
	sig := inspectState(state)

	jailbreakP := 0.03
	ragInjP := 0.02
	credP := 0.04
	harmScore := 0.18
	harmProbs := map[string]float64{"0": 0.84, "1": 0.14, "2": 0.02}

	if sig.Injection {
		jailbreakP = 0.94
		ragInjP = 0.97
		harmScore = 1.88
		harmProbs = map[string]float64{"0": 0.02, "1": 0.08, "2": 0.90}
	}
	if sig.CredentialLeak {
		credP = 0.93
		if !sig.Injection {
			harmScore = 1.62
			harmProbs = map[string]float64{"0": 0.05, "1": 0.28, "2": 0.67}
		}
	}

	vendorP := band(sig.VendorSupported, 0.97, 0.12)
	amountP := band(sig.AmountSupported, 0.96, 0.11)
	dateP := band(sig.DateSupported, 0.95, 0.09)
	claimP := band(sig.ClaimSupported, 0.94, 0.07)
	if sig.Injection {
		vendorP = 0.41
		amountP = 0.38
		dateP = 0.36
		claimP = 0.05
	}

	citationChoice := "verbatim_supported"
	citationConf := 0.92
	citationProbs := map[string]float64{"verbatim_supported": 0.93, "extrapolated": 0.04, "contradicted": 0.02, "unverifiable": 0.01}
	if sig.HallucinatedDate {
		citationChoice = "extrapolated"
		citationConf = 0.88
		citationProbs = map[string]float64{"verbatim_supported": 0.07, "extrapolated": 0.86, "contradicted": 0.05, "unverifiable": 0.02}
	}
	if sig.Injection {
		citationChoice = "contradicted"
		citationConf = 0.91
		citationProbs = map[string]float64{"verbatim_supported": 0.03, "extrapolated": 0.08, "contradicted": 0.84, "unverifiable": 0.05}
	}

	execTier := "tier1_fast_mini"
	execConf := 0.90
	execProbs := map[string]float64{"tier0_deterministic": 0.04, "tier1_fast_mini": 0.91, "tier2_frontier_reasoning": 0.04, "tier3_human_escalation": 0.01}
	skill := "none_needed"
	skillConf := 0.89
	skillProbs := map[string]float64{"none_needed": 0.90, "sql_analytics_ro": 0.03, "billing_refund_exec": 0.03, "sec_edgar_verifier": 0.02, "incident_pager_alert": 0.02}

	switch {
	case sig.Injection || sig.CredentialLeak:
		execTier = "tier0_deterministic"
		execConf = 0.96
		execProbs = map[string]float64{"tier0_deterministic": 0.96, "tier1_fast_mini": 0.01, "tier2_frontier_reasoning": 0.01, "tier3_human_escalation": 0.02}
		skill = "incident_pager_alert"
		skillConf = 0.62
		skillProbs = map[string]float64{"none_needed": 0.20, "sql_analytics_ro": 0.02, "billing_refund_exec": 0.02, "sec_edgar_verifier": 0.02, "incident_pager_alert": 0.74}
	case sig.HallucinatedDate:
		execTier = "tier2_frontier_reasoning"
		execConf = 0.87
		execProbs = map[string]float64{"tier0_deterministic": 0.02, "tier1_fast_mini": 0.08, "tier2_frontier_reasoning": 0.87, "tier3_human_escalation": 0.03}
		skill = "sec_edgar_verifier"
		skillConf = 0.84
		skillProbs = map[string]float64{"none_needed": 0.06, "sql_analytics_ro": 0.04, "billing_refund_exec": 0.02, "sec_edgar_verifier": 0.86, "incident_pager_alert": 0.02}
	case sig.DuplicateRefund:
		execTier = "tier0_deterministic"
		execConf = 0.94
		execProbs = map[string]float64{"tier0_deterministic": 0.94, "tier1_fast_mini": 0.04, "tier2_frontier_reasoning": 0.01, "tier3_human_escalation": 0.01}
		skill = "billing_refund_exec"
		skillConf = 0.95
		skillProbs = map[string]float64{"none_needed": 0.02, "sql_analytics_ro": 0.01, "billing_refund_exec": 0.95, "sec_edgar_verifier": 0.01, "incident_pager_alert": 0.01}
	}

	_ = pipeline
	rawJSON, _ := json.Marshal(state)
	inTok := maxInt(len(rawJSON)/3+340, 520)
	outTok := 64

	return &typesafe.SystemOneResponse{
		Model:     typesafe.ModelJev1_13_0,
		RequestID: fmt.Sprintf("req_sim_%s", time.Now().UTC().Format("150405.000")),
		Usage: typesafe.Usage{
			InputTokens:  &inTok,
			OutputTokens: &outTok,
		},
		Nouls: applySimulatedFieldNouls(map[string]typesafe.NoulResponse{
			qJailbreak:     {Type: "noul", Noul: jailbreakP},
			qRAGInjection:  {Type: "noul", Noul: ragInjP},
			qCredentialPII: {Type: "noul", Noul: credP},
			qFieldVendor:   {Type: "noul", Noul: vendorP},
			qFieldAmount:   {Type: "noul", Noul: amountP},
			qFieldDate:     {Type: "noul", Noul: dateP},
			qFieldClaim:    {Type: "noul", Noul: claimP},
		}, specs, state, sig),
		Choices: map[string]typesafe.ChoiceResponse[string]{
			qExecTier:   {Type: "choice", Choice: execTier, Confidence: execConf, Probabilities: execProbs},
			qAgentSkill: {Type: "choice", Choice: skill, Confidence: skillConf, Probabilities: skillProbs},
			qCitation:   {Type: "choice", Choice: citationChoice, Confidence: citationConf, Probabilities: citationProbs},
		},
		Scores: map[string]typesafe.ScoreResponse{
			qHarmSeverity: {Type: "score", Score: harmScore, Confidence: 0.91, Probabilities: harmProbs},
		},
	}
}

func applySimulatedFieldNouls(nouls map[string]typesafe.NoulResponse, specs []BoundQuestionSpec, state any, sig stateSignals) map[string]typesafe.NoulResponse {
	if nouls == nil {
		nouls = map[string]typesafe.NoulResponse{}
	}
	extraction := extractionObject(state)
	for _, spec := range specs {
		if spec.Primitive != "noul" || !isFieldQuestion(spec.Key) {
			continue
		}
		if _, exists := nouls[spec.Key]; exists && (spec.Key == qFieldVendor || spec.Key == qFieldAmount || spec.Key == qFieldDate || spec.Key == qFieldClaim) {
			continue
		}
		name := spec.FieldName
		if name == "" {
			name = spec.Key
		}
		nl := strings.ToLower(name)
		ok := false
		switch {
		case strings.Contains(nl, "vendor") || strings.Contains(nl, "entity") || strings.Contains(nl, "subject"):
			ok = sig.VendorSupported
		case strings.Contains(nl, "amount") || strings.Contains(nl, "value") || strings.Contains(nl, "fine"):
			ok = sig.AmountSupported
		case strings.Contains(nl, "date") || strings.Contains(nl, "deadline") || strings.Contains(nl, "notice"):
			ok = sig.DateSupported && !sig.HallucinatedDate
		case strings.Contains(nl, "claim") || strings.Contains(nl, "renew"):
			ok = sig.ClaimSupported
		default:
			val := stringify(firstValue(extraction, name))
			ok = val != "" && (sourceSupportsEntity(sig.SourceText, val) || strings.Contains(sig.SourceText, strings.ToLower(val)))
		}
		if sig.Injection {
			ok = false
		}
		nouls[spec.Key] = typesafe.NoulResponse{Type: "noul", Noul: band(ok, 0.95, 0.10)}
	}
	return nouls
}

func band(ok bool, high, low float64) float64 {
	if ok {
		return high
	}
	return low
}

func asObject(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	default:
		return map[string]any{}
	}
}

func asSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []map[string]any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out
	default:
		return nil
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		n, _ := t.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.ReplaceAll(t, ",", ""), 64)
		return n
	default:
		return 0
	}
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, _ := json.Marshal(t)
		return strings.Trim(string(b), `"`)
	}
}

func firstString(obj map[string]any, keys ...string) string {
	return stringify(firstValue(obj, keys...))
}

func firstValue(obj map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := obj[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func containsAny(haystack string, needles ...string) bool {
	h := strings.ToLower(haystack)
	for _, n := range needles {
		if n != "" && strings.Contains(h, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func collectPassageText(v any) string {
	var parts []string
	for _, item := range asSlice(v) {
		obj := asObject(item)
		if t := stringify(obj["text"]); t != "" {
			parts = append(parts, t)
		}
	}
	if typed, ok := v.([]map[string]any); ok {
		for _, obj := range typed {
			if t := stringify(obj["text"]); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

var nonDigit = regexp.MustCompile(`\D+`)

func digitsOnly(s string) string {
	return nonDigit.ReplaceAllString(s, "")
}
