package aegiscortex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var daysPriorRE = regexp.MustCompile(`(?i)(?:at least\s+)?(?:thirty|\d+)\s*(?:\((\d+)\)\s*)?days?\s+prior`)

// FieldPatch is one surgically repaired extraction.
type FieldPatch struct {
	Field    string `json:"field"`
	Before   any    `json:"before"`
	After    any    `json:"after"`
	Method   string `json:"method"`
	Evidence string `json:"evidence"`
}

// FrozenField is a Jev-locked extraction that must not be regenerated.
type FrozenField struct {
	Field   string  `json:"field"`
	YesProb float64 `json:"yes_prob"`
	Value   any     `json:"value,omitempty"`
}

// SurgicalPatch is the Tier-2 repair artifact: prompt, locked JSON, spliced result.
type SurgicalPatch struct {
	FailedFields    []string       `json:"failed_fields"`
	Prompt          string         `json:"prompt"`
	MicroPrompt     string         `json:"micro_prompt"`
	TokenBudget     int            `json:"token_budget"`
	FullRetryTokens int            `json:"full_retry_tokens"`
	FrozenFields    []FrozenField  `json:"frozen_fields,omitempty"`
	LockedJSON      map[string]any `json:"locked_json"`
	RepairedJSON    map[string]any `json:"repaired_json"`
	Patches         []FieldPatch   `json:"patches"`
	ModeledCostUSD  float64        `json:"modeled_cost_usd"`
	Executed        bool           `json:"executed"`
}

func extractionObject(state any) map[string]any {
	root := asObject(state)
	if m := asObject(root["mini_model_extraction"]); len(m) > 0 {
		return cloneMap(m)
	}
	if m := asObject(root["extracted_fields"]); len(m) > 0 {
		return cloneMap(m)
	}
	return map[string]any{}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildSurgicalPatch(state any, failed []string, fieldViews []FieldVerificationView) *SurgicalPatch {
	if len(failed) == 0 {
		return nil
	}
	root := asObject(state)
	source := firstString(root, "source_document", "refund_policy")
	if source == "" {
		source = collectPassageText(root["retrieved_passages"])
	}
	locked := extractionObject(state)
	repaired := cloneMap(locked)
	patches := make([]FieldPatch, 0, len(failed))

	failedSet := map[string]bool{}
	lockedNames := map[string]bool{}
	for _, name := range failed {
		failedSet[strings.ToLower(name)] = true
	}
	for _, fv := range fieldViews {
		if fv.Verified {
			lockedNames[strings.ToLower(fv.FieldName)] = true
			continue
		}
		failedSet[strings.ToLower(fv.FieldName)] = true
	}

	lockKeys := sortedKeys(locked)
	for _, key := range lockKeys {
		before := locked[key]
		if lockedNames[strings.ToLower(key)] {
			continue
		}
		if !failedSet[strings.ToLower(key)] && !failedMatches(failed, key) {
			continue
		}
		after, method, evidence := repairField(source, locked, key, before)
		repaired[key] = after
		patches = append(patches, FieldPatch{
			Field:    key,
			Before:   before,
			After:    after,
			Method:   method,
			Evidence: evidence,
		})
	}
	for _, name := range failed {
		if _, ok := repaired[name]; ok {
			continue
		}
		if patchCovers(patches, name) {
			continue
		}
		after, method, evidence := repairField(source, locked, name, nil)
		if method == "unresolved" && after == nil {
			continue
		}
		repaired[name] = after
		patches = append(patches, FieldPatch{
			Field:    name,
			Before:   nil,
			After:    after,
			Method:   method,
			Evidence: evidence,
		})
	}

	lockedOnly := map[string]any{}
	for k, v := range locked {
		kl := strings.ToLower(k)
		if lockedNames[kl] || (!failedSet[kl] && !failedMatches(failed, k)) {
			lockedOnly[k] = v
		}
	}

	prompt := surgicalPrompt(source, lockedOnly, failed)
	micro := microRepairPrompt(source, failed, patches)
	frozen := make([]FrozenField, 0, len(fieldViews))
	for _, fv := range fieldViews {
		if !fv.Verified {
			continue
		}
		frozen = append(frozen, FrozenField{
			Field:   fv.FieldName,
			YesProb: fv.YesProb,
			Value:   firstValue(locked, fv.FieldName),
		})
	}
	return &SurgicalPatch{
		FailedFields:    failed,
		Prompt:          prompt,
		MicroPrompt:     micro,
		TokenBudget:     tokenEstimate(micro),
		FullRetryTokens: 1420,
		FrozenFields:    frozen,
		LockedJSON:      lockedOnly,
		RepairedJSON:    repaired,
		Patches:         patches,
		ModeledCostUSD:  modeledFrontierUSD,
		Executed:        len(patches) > 0,
	}
}

func tokenEstimate(s string) int {
	n := len(strings.Fields(s))
	if n < 12 {
		return 12
	}
	return n
}

func microRepairPrompt(source string, failed []string, patches []FieldPatch) string {
	clip := source
	if len(clip) > 180 {
		clip = clip[:180] + "…"
	}
	target := strings.Join(failed, ", ")
	if target == "" && len(patches) > 0 {
		names := make([]string, 0, len(patches))
		for _, p := range patches {
			names = append(names, p.Field)
		}
		target = strings.Join(names, ", ")
	}
	return fmt.Sprintf("Given %q, compute only %s. Do not regenerate locked fields.", clip, target)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func patchCovers(patches []FieldPatch, name string) bool {
	nl := strings.ToLower(name)
	for _, p := range patches {
		pl := strings.ToLower(p.Field)
		if pl == nl || strings.Contains(pl, nl) || strings.Contains(nl, pl) {
			return true
		}
		if containsAny(nl, "date", "notice", "deadline") && containsAny(pl, "date", "notice", "deadline") {
			return true
		}
	}
	return false
}

func failedMatches(failed []string, key string) bool {
	kl := strings.ToLower(key)
	for _, f := range failed {
		fl := strings.ToLower(f)
		if fl == kl || strings.Contains(fl, kl) || strings.Contains(kl, fl) {
			return true
		}
		if strings.Contains(fl, "notice") && (strings.Contains(kl, "notice") || strings.Contains(kl, "deadline")) {
			return true
		}
	}
	return false
}

func repairField(source string, extraction map[string]any, key string, before any) (any, string, string) {
	kl := strings.ToLower(key)
	src := source
	if strings.Contains(kl, "date") || strings.Contains(kl, "deadline") || strings.Contains(kl, "notice") {
		if after, ok := repairRelativeNoticeDate(source, extraction); ok {
			return after, "relative_date", "Anniversary of Effective Date minus the contractual notice window."
		}
	}
	if strings.Contains(kl, "vendor") || strings.Contains(kl, "entity") || strings.Contains(kl, "subject") {
		if name := extractVendor(src); name != "" {
			return name, "source_verbatim", name
		}
	}
	if strings.Contains(kl, "amount") || strings.Contains(kl, "value") || strings.Contains(kl, "fine") {
		if amt := extractAmount(src); amt != "" {
			return amt, "source_verbatim", amt
		}
	}
	beforeStr := stringify(before)
	if beforeStr != "" && strings.Contains(strings.ToLower(src), strings.ToLower(beforeStr)) {
		return before, "source_verbatim", beforeStr
	}
	return before, "unresolved", "No deterministic splice from the source; keep the failed value flagged."
}

func repairRelativeNoticeDate(source string, extraction map[string]any) (string, bool) {
	days := 30
	if m := daysPriorRE.FindStringSubmatch(source); len(m) > 0 {
		if m[1] != "" {
			if n := int(asFloat(m[1])); n > 0 {
				days = n
			}
		} else if containsAny(strings.ToLower(m[0]), "thirty") {
			days = 30
		}
	} else if !containsAny(strings.ToLower(source), "days prior", "days before") {
		return "", false
	}

	effectiveRaw := stringify(firstValue(extraction, "effective_date", "effective_or_notice_date"))
	var effective time.Time
	var ok bool
	if effectiveRaw != "" {
		effective, ok = parseFlexibleDate(effectiveRaw)
	}
	if !ok {
		effective, ok = parseFlexibleDateFromSource(source)
	}
	if !ok {
		return "", false
	}

	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	ann := time.Date(now.Year(), effective.Month(), effective.Day(), 0, 0, 0, 0, time.UTC)
	if !ann.After(now) {
		ann = ann.AddDate(1, 0, 0)
	}
	deadline := ann.AddDate(0, 0, -days)
	return deadline.Format("01/02/2006"), true
}

func parseFlexibleDateFromSource(source string) (time.Time, bool) {
	// "Effective Date (November 1, 2025)" or similar.
	re := regexp.MustCompile(`(?i)((?:January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},\s+\d{4})`)
	if m := re.FindStringSubmatch(source); len(m) > 1 {
		return parseFlexibleDate(m[1])
	}
	re = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
	if m := re.FindStringSubmatch(source); len(m) > 1 {
		return parseFlexibleDate(m[1])
	}
	return time.Time{}, false
}

func extractVendor(source string) string {
	re := regexp.MustCompile(`(?i)(?:vendor|contracting vendor|party)\s+is\s+([A-Z][A-Za-z0-9&., ]+?(?:LLC|Inc|Ltd|GmbH|Corp)\.?)`)
	if m := re.FindStringSubmatch(source); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractAmount(source string) string {
	re := regexp.MustCompile(`(?i)(?:USD|EUR|€|\$)\s*([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]+)?)`)
	if m := re.FindStringSubmatch(source); len(m) > 1 {
		return strings.TrimSpace(m[0])
	}
	return ""
}

func surgicalPrompt(source string, locked map[string]any, failed []string) string {
	lockJSON, _ := json.MarshalIndent(locked, "", "  ")
	var b strings.Builder
	b.WriteString("Repair ONLY the failed fields. Copy locked fields unchanged.\n")
	b.WriteString("Return a JSON object whose keys are exactly the failed field names.\n\n")
	fmt.Fprintf(&b, "Failed fields: %s\n\n", strings.Join(failed, ", "))
	b.WriteString("Locked JSON (do not regenerate):\n")
	b.Write(lockJSON)
	b.WriteString("\n\nSource:\n")
	if len(source) > 900 {
		source = source[:900] + "…"
	}
	b.WriteString(source)
	b.WriteString("\n")
	return b.String()
}
