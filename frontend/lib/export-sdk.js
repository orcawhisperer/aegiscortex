export function exportSnippets({ thresholds, questions, pipeline }) {
  const sec = Number(thresholds.security_gate_confidence).toFixed(2);
  const field = Number(thresholds.field_verify_confidence).toFixed(2);
  const route = Number(thresholds.router_confidence).toFixed(2);
  const comp = Number(thresholds.composite_pass_threshold).toFixed(0);
  const keys = (questions || []).map((q) => q.id).filter(Boolean);
  const fieldKeys = keys.filter((k) => k.startsWith("verify_field_"));

  const go = `// AegisCortex control-plane gates — calibrated in the studio
tauSec, tauField, tauRoute, tauComp := ${sec}, ${field}, ${route}, ${comp}.0
questions := typesafe.BindQuestions(
    typesafe.DefineNoul("jailbreak_attempt", "…", noulCrit),
    typesafe.DefineNoul("indirect_prompt_injection", "…", noulCrit),
    typesafe.DefineNoul("credential_or_pii_exposure", "…", noulCrit),
    typesafe.DefineScore("policy_harm_severity", "…", "Negligible", "Moderate", "Severe"),
    typesafe.DefineChoice("execution_tier", "…", tiers),
    typesafe.DefineChoice("selected_agent_skill", "…", skills),
    typesafe.DefineChoice("citation_grounding", "…", citations),
${fieldKeys.map((k) => `    typesafe.DefineNoul("${k}", "Is extracted field supported by the source?", noulCrit),`).join("\n")}
)
resp, _ := scorer.Score(ctx, state, questions)
if noul, _ := typesafe.GetNoul(resp, "indirect_prompt_injection"); noul.Noul >= tauSec {
    return "TIER_0_BLOCK"
}
dec := typesafe.RouteNoul(noul, tauField, 0.30)
_ = dec
_ = tauRoute
_ = tauComp
`;

  const ts = `const tau = { sec: ${sec}, field: ${field}, route: ${route}, comp: ${comp} };
const questions = ${JSON.stringify(keys)};
// Bind the same matrix in typesafe-sdk (TS) and call your SpeculativeScorer.
export function route(noul, tauField = tau.field) {
  return noul >= tauField ? "lock" : "repair";
}
`;

  const curl = `curl -sS -X POST "$ORIGIN/svc/api/evaluate" \\
  -H "Content-Type: application/json" \\
  -d '{
    "scenario_id": "${pipeline || "rag_citation"}",
    "thresholds": {
      "security_gate_confidence": ${sec},
      "field_verify_confidence": ${field},
      "router_confidence": ${route},
      "composite_pass_threshold": ${comp}
    },
    "context": {}
  }'`;

  return { go, ts, curl };
}
