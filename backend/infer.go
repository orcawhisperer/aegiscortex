package aegiscortex

import "strings"

func isCanonicalExtractionKey(name string) bool {
	n := strings.ToLower(name)
	switch {
	case containsAny(n, "vendor", "entity", "legal_subject", "party"):
		return true
	case containsAny(n, "amount", "value_usd", "fine", "price", "invoice"):
		return true
	case containsAny(n, "date", "deadline", "notice", "window"):
		return true
	case containsAny(n, "claim", "renew", "entail", "grounding"):
		return true
	default:
		return false
	}
}

// InferExtraFields synthesizes field nouls for extraction keys that are
// not already covered by the default vendor/amount/date/claim quartet.
func InferExtraFields(state any) []CompiledField {
	ext := extractionObject(state)
	if len(ext) == 0 {
		return nil
	}
	out := make([]CompiledField, 0, 4)
	for name, val := range ext {
		if isCanonicalExtractionKey(name) {
			continue
		}
		out = append(out, CompiledField{
			Name:        name,
			JSONPath:    name,
			QuestionKey: fieldQuestionKey(name),
			Type:        inferJSONType(val),
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}
