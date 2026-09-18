package aegiscortex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/orcawhisperer/typesafe-sdk-go"
)

const maxCompiledFields = 16

var (
	identCleaner = regexp.MustCompile(`[^a-z0-9_]+`)
	tsFieldLine  = regexp.MustCompile(`(?m)^\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\??\s*:\s*([A-Za-z0-9_\[\]'"| ]+)`)
)

var contextPropertySkip = map[string]bool{
	"user_prompt": true, "source_document": true, "draft_reply": true,
	"retrieved_passages": true, "ticket": true, "customer_orders": true,
	"refund_policy": true, "target_schema": true, "context": true,
}

// CompiledField is one leaf property turned into a field noul.
type CompiledField struct {
	Name        string `json:"name"`
	JSONPath    string `json:"json_path"`
	QuestionKey string `json:"question_key"`
	Type        string `json:"type"`
}

// CompiledSchema is the BindQuestions matrix derived from a user schema.
type CompiledSchema struct {
	Title  string          `json:"title,omitempty"`
	Fields []CompiledField `json:"fields"`
	Stub   string          `json:"stub,omitempty"`
}

// CompileSchemaText accepts JSON Schema, a JSON object example, or a TypeScript interface.
func CompileSchemaText(text string) (CompiledSchema, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return CompiledSchema{}, fmt.Errorf("schema is empty")
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		var raw any
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			return CompiledSchema{}, fmt.Errorf("invalid JSON schema: %w", err)
		}
		obj, _ := raw.(map[string]any)
		if obj == nil {
			return CompiledSchema{}, fmt.Errorf("schema must be a JSON object")
		}
		return CompileJSONSchema(obj)
	}
	return compileTypeScriptInterface(text)
}

// CompileJSONSchema walks properties (and nested extraction objects) into field nouls.
func CompileJSONSchema(schema map[string]any) (CompiledSchema, error) {
	if schema == nil {
		return CompiledSchema{}, fmt.Errorf("schema is empty")
	}
	title := stringify(schema["title"])
	fields := collectSchemaFields(schema, "")
	if len(fields) == 0 {
		return CompiledSchema{}, fmt.Errorf("no object properties found")
	}
	if len(fields) > maxCompiledFields {
		fields = fields[:maxCompiledFields]
	}
	used := map[string]int{}
	for i := range fields {
		key := fieldQuestionKey(fields[i].Name)
		if n := used[key]; n > 0 {
			key = fmt.Sprintf("%s_%d", key, n+1)
		}
		used[fieldQuestionKey(fields[i].Name)]++
		fields[i].QuestionKey = key
	}
	out := CompiledSchema{Title: title, Fields: fields}
	out.Stub = bindQuestionsStub(out)
	return out, nil
}

func collectSchemaFields(node map[string]any, prefix string) []CompiledField {
	props, _ := node["properties"].(map[string]any)
	if props == nil {
		// Example instance: treat own keys as fields.
		if !looksLikeSchemaNode(node) {
			props = node
		}
	}
	if props == nil {
		return nil
	}

	// Prefer extraction envelopes when present.
	for _, wrap := range []string{"mini_model_extraction", "extracted_fields"} {
		if raw, ok := props[wrap]; ok {
			child := asObject(raw)
			if nested := collectSchemaFields(child, wrap); len(nested) > 0 {
				return nested
			}
		}
	}

	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	var fields []CompiledField
	for _, name := range names {
		if contextPropertySkip[name] || strings.HasPrefix(name, "$") {
			continue
		}
		child := asObject(props[name])
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if childType := strings.ToLower(stringify(child["type"])); childType == "object" || child["properties"] != nil {
			if nested := collectSchemaFields(child, path); len(nested) > 0 {
				fields = append(fields, nested...)
				continue
			}
		}
		typ := strings.ToLower(stringify(child["type"]))
		if typ == "" {
			typ = inferJSONType(props[name])
		}
		if typ == "array" || typ == "object" {
			continue
		}
		fields = append(fields, CompiledField{
			Name:     name,
			JSONPath: path,
			Type:     typ,
		})
	}
	return fields
}

func looksLikeSchemaNode(node map[string]any) bool {
	_, hasProps := node["properties"]
	_, hasSchema := node["$schema"]
	_, hasType := node["type"]
	return hasProps || hasSchema || hasType
}

func inferJSONType(v any) string {
	switch v.(type) {
	case bool:
		return "boolean"
	case float64, float32, int, int64, json.Number:
		return "number"
	case string:
		return "string"
	default:
		return "string"
	}
}

func compileTypeScriptInterface(src string) (CompiledSchema, error) {
	title := "CustomSchema"
	if m := regexp.MustCompile(`(?m)(?:interface|type)\s+([A-Za-z_][A-Za-z0-9_]*)`).FindStringSubmatch(src); len(m) > 1 {
		title = m[1]
	}
	matches := tsFieldLine.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return CompiledSchema{}, fmt.Errorf("no TypeScript fields found")
	}
	fields := make([]CompiledField, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		name := m[1]
		if seen[name] || name == "interface" || name == "type" {
			continue
		}
		seen[name] = true
		typ := strings.ToLower(strings.TrimSpace(m[2]))
		switch {
		case strings.Contains(typ, "number") || strings.Contains(typ, "int"):
			typ = "number"
		case strings.Contains(typ, "bool"):
			typ = "boolean"
		default:
			typ = "string"
		}
		fields = append(fields, CompiledField{
			Name:        name,
			JSONPath:    name,
			QuestionKey: fieldQuestionKey(name),
			Type:        typ,
		})
		if len(fields) >= maxCompiledFields {
			break
		}
	}
	out := CompiledSchema{Title: title, Fields: fields}
	out.Stub = bindQuestionsStub(out)
	return out, nil
}

func fieldQuestionKey(name string) string {
	ident := identCleaner.ReplaceAllString(strings.ToLower(name), "_")
	ident = strings.Trim(ident, "_")
	if ident == "" {
		ident = "field"
	}
	return "verify_field_" + ident
}

func fieldNoulPrompt(field CompiledField) string {
	switch field.Type {
	case "number":
		return fmt.Sprintf("Is extracted field %q (numeric) present verbatim in the source document, not rounded or invented?", field.Name)
	case "boolean":
		return fmt.Sprintf("Is extracted field %q (boolean) entailed by an explicit statement in the source?", field.Name)
	default:
		if strings.Contains(strings.ToLower(field.Name), "date") || field.Type == "date" {
			return fmt.Sprintf("Is extracted field %q a calendar value stated in the source, not a hallucinated date computed from a relative window?", field.Name)
		}
		return fmt.Sprintf("Is extracted field %q present verbatim or by unambiguous alias in the source document?", field.Name)
	}
}

func bindQuestionsStub(schema CompiledSchema) string {
	var b strings.Builder
	b.WriteString("// Generated BindQuestions matrix — security + router + per-field nouls\n")
	b.WriteString("questions := typesafe.BindQuestions(\n")
	b.WriteString("    typesafe.DefineNoul(\"jailbreak_attempt\", \"…\", noulCrit),\n")
	b.WriteString("    typesafe.DefineNoul(\"indirect_prompt_injection\", \"…\", noulCrit),\n")
	b.WriteString("    typesafe.DefineNoul(\"credential_or_pii_exposure\", \"…\", noulCrit),\n")
	b.WriteString("    typesafe.DefineScore(\"policy_harm_severity\", \"…\", \"Negligible\", \"Moderate\", \"Severe\"),\n")
	b.WriteString("    typesafe.DefineChoice(\"execution_tier\", \"…\", tiers),\n")
	b.WriteString("    typesafe.DefineChoice(\"selected_agent_skill\", \"…\", skills),\n")
	b.WriteString("    typesafe.DefineChoice(\"citation_grounding\", \"…\", citations),\n")
	for _, f := range schema.Fields {
		fmt.Fprintf(&b, "    typesafe.DefineNoul(%q, %q, typesafe.NoulCriteria{True: \"verbatim in source\", False: \"missing or invented\"}),\n",
			f.QuestionKey, fieldNoulPrompt(f))
	}
	b.WriteString(")\n")
	return b.String()
}

func compiledFieldQuestions(fields []CompiledField) []typesafe.BoundQuestion {
	out := make([]typesafe.BoundQuestion, 0, len(fields))
	for _, f := range fields {
		out = append(out, typesafe.DefineNoul(f.QuestionKey, fieldNoulPrompt(f), typesafe.NoulCriteria{
			True:  "The extracted value is present verbatim or by unambiguous alias in the source",
			False: "The extracted value is missing, invented, or not entailed by the source",
		}))
	}
	return out
}

func compiledFieldSpecs(fields []CompiledField) []BoundQuestionSpec {
	out := make([]BoundQuestionSpec, 0, len(fields))
	for _, f := range fields {
		out = append(out, BoundQuestionSpec{
			Key:          f.QuestionKey,
			Primitive:    "noul",
			Category:     "Field",
			Instructions: fieldNoulPrompt(f),
			FieldName:    f.Name,
		})
	}
	return out
}
