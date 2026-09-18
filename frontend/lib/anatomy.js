const SEC_IDS = ["jailbreak_attempt", "indirect_prompt_injection", "credential_or_pii_exposure"];
const ROUTER_IDS = ["execution_tier", "selected_agent_skill"];

export function anatomyModel(evalRes, thresholds) {
  const questions = evalRes?.questions || [];
  const fields = evalRes?.field_verifications || [];
  const q = (id) => questions.find((x) => x.id === id) || {};
  const tauSec = Number(thresholds?.security_gate_confidence ?? 0.65);
  const tauField = Number(thresholds?.field_verify_confidence ?? 0.75);
  const tauRoute = Number(thresholds?.router_confidence ?? 0.8);
  const risk = (evalRes?.composite_risk || 0) / 100;
  const maxSec = Math.max(risk, ...SEC_IDS.map((id) => q(id).top_probability || 0));
  const exec = q("execution_tier");
  const locked = fields.filter((f) => f.verified).length;
  const failed = fields.filter((f) => !f.verified).length;
  const guardrail = SEC_IDS.some((id) => (q(id).top_probability || 0) >= tauSec) || risk >= tauSec;

  return {
    heads: evalRes?.question_count || questions.length || 11,
    cost: evalRes?.cascade?.aegis_control_cost_usd ?? 0.000041,
    latency: evalRes?.cascade?.reference_jev_p50_ms ?? 114,
    security: {
      count: SEC_IDS.filter((id) => q(id).id || questions.some((row) => row.id === id)).length || 4,
      risk: maxSec,
      tau: tauSec,
      pass: !guardrail,
    },
    router: {
      count: ROUTER_IDS.filter((id) => questions.some((row) => row.id === id)).length || 2,
      conf: exec.confidence || exec.top_probability || 0,
      tau: tauRoute,
      pass: (exec.confidence || 0) >= tauRoute || (evalRes?.final_route_tier || "").includes("FASTPATH") || (evalRes?.final_route_tier || "").includes("AUTO"),
    },
    fields: {
      count: fields.length || 5,
      locked,
      failed,
      tau: tauField,
    },
    route: evalRes?.final_route_tier || "",
  };
}
