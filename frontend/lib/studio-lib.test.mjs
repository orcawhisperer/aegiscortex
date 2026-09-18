import assert from "node:assert/strict";
import { highlightPayload, needleForRow } from "./highlight.js";
import { exportSnippets } from "./export-sdk.js";
import { parseStudioParams } from "./permalink.js";
import { watchFor, ROUTE_TO_CASE } from "./watch-copy.js";
import { anatomyModel } from "./anatomy.js";

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

assert.match(watchFor("gw_rag_injection").ribbon, /SYSTEM OVERRIDE/);
assert.equal(ROUTE_TO_CASE.TIER_2_SURGICAL_FIELD_REPAIR, "sde_hallucinated_date");

const anatomy = anatomyModel(
  {
    question_count: 11,
    composite_risk: 3.7,
    final_route_tier: "TIER_2_SURGICAL_FIELD_REPAIR",
    cascade: { aegis_control_cost_usd: 0.000041, reference_jev_p50_ms: 114 },
    questions: [
      { id: "indirect_prompt_injection", top_probability: 0.04 },
      { id: "execution_tier", confidence: 0.87 },
    ],
    field_verifications: [
      { verified: true },
      { verified: true },
      { verified: true },
      { verified: false },
    ],
  },
  { security_gate_confidence: 0.65, field_verify_confidence: 0.75, router_confidence: 0.8 }
);
assert.equal(anatomy.fields.locked, 3);
assert.equal(anatomy.fields.failed, 1);
assert.equal(anatomy.security.pass, true);
assert.equal(anatomy.route, "TIER_2_SURGICAL_FIELD_REPAIR");

console.log("studio-lib ok");
