import assert from "node:assert/strict";
import { highlightPayload, needleForRow } from "./highlight.js";
import { exportSnippets } from "./export-sdk.js";
import { parseStudioParams } from "./permalink.js";

const marked = highlightPayload('{"hidden":"SYSTEM OVERRIDE"}', "SYSTEM OVERRIDE");
assert.equal(marked.filter((p) => p.mark).length, 1);
assert.equal(marked.find((p) => p.mark).text, "SYSTEM OVERRIDE");

assert.equal(
  needleForRow("indirect_prompt_injection", [], '{"note":"SYSTEM OVERRIDE"}'),
  "SYSTEM OVERRIDE"
);
assert.equal(
  needleForRow("verify_field_invoice_date", [{ key: "notice_deadline_date", needle: "09/15/2026" }], '{"d":"09/15/2026"}'),
  "09/15/2026"
);
assert.equal(
  needleForRow(
    "vendor_or_entity_id",
    [],
    JSON.stringify({ mini_model_extraction: { vendor_name: "Northwind Analytics LLC" } })
  ),
  "Northwind Analytics LLC"
);

const parsed = parseStudioParams({
  case: "sde_hallucinated_date",
  sec: "0.70",
  field: "0.80",
  route: "0.85",
  comp: "72",
});
assert.equal(parsed.case, "sde_hallucinated_date");
assert.equal(parsed.thresholds.security_gate_confidence, 0.7);
assert.equal(parsed.thresholds.composite_pass_threshold, 72);

const empty = parseStudioParams({});
assert.equal(empty.case, "");
assert.equal(empty.thresholds, null);

const snippets = exportSnippets({
  thresholds: {
    security_gate_confidence: 0.65,
    field_verify_confidence: 0.75,
    router_confidence: 0.8,
    composite_pass_threshold: 68,
  },
  questions: [{ id: "verify_field_governing_law" }],
  pipeline: "sde_cascade",
});
assert.match(snippets.go, /tauSec, tauField, tauRoute, tauComp := 0.65, 0.75, 0.80, 68.0/);
assert.match(snippets.go, /verify_field_governing_law/);
assert.match(snippets.ts, /field: 0.75/);
assert.match(snippets.curl, /sde_cascade/);

console.log("studio-lib ok");
