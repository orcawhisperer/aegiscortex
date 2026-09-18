function replayRuling(evalRes, thresholds) {
  if (!evalRes) return null;
  const tauSec = Number(thresholds.security_gate_confidence);
  const tauField = Number(thresholds.field_verify_confidence);
  const tauRoute = Number(thresholds.router_confidence);
  const tauComp = Number(thresholds.composite_pass_threshold);
  const questions = evalRes.questions || [];
  const q = (id) => questions.find((x) => x.id === id) || {};

  const fields = (evalRes.field_verifications || []).map((f) => {
    const verified = f.yes_prob >= tauField;
    return {
      ...f,
      verified,
      action: verified ? "Verified by field noul — locked" : "Failed field noul — surgical repair this field only",
    };
  });
  const failed = fields.filter((f) => !f.verified).map((f) => f.field_name);

  const guardrail =
    (q("jailbreak_attempt").top_probability || 0) >= tauSec ||
    (q("indirect_prompt_injection").top_probability || 0) >= tauSec ||
    (q("credential_or_pii_exposure").top_probability || 0) >= tauSec;
  const risk = (evalRes.composite_risk || 0) / 100;
  const trust = evalRes.composite_score || 0;
  const citation = q("citation_grounding").selected_choice;
  const exec = q("execution_tier");
  const skill = q("selected_agent_skill").selected_choice;

  let final_route_tier = "TIER_1_VERIFIED_FASTPATH";
  if (guardrail || risk >= tauSec) {
    final_route_tier = "TIER_0_BLOCK";
  } else if (failed.length || citation === "contradicted" || citation === "extrapolated") {
    final_route_tier = "TIER_2_SURGICAL_FIELD_REPAIR";
  } else if (trust < tauComp) {
    final_route_tier = "TIER_2_SURGICAL_FIELD_REPAIR";
  } else if (
    exec.selected_choice === "tier0_deterministic" &&
    (exec.confidence || 0) >= tauRoute &&
    skill !== "none_needed" &&
    skill !== "incident_pager_alert"
  ) {
    final_route_tier = "TIER_0_AUTO_EXEC";
  }

  return {
    ...evalRes,
    final_route_tier,
    failed_fields: failed,
    field_verifications: fields,
    replayed: true,
  };
}

function raceModel(tier) {
  const control = 114;
  const stream = 2400;
  const abort = tier === "TIER_0_BLOCK" || tier === "TIER_0_AUTO_EXEC";
  return {
    control,
    stream,
    abortAt: abort ? control : null,
    wasted: abort ? Math.max(0, stream - control) : 0,
    caption: abort
      ? "Modeled race: control plane aborts the downstream stream at 114 ms. No live LLM is streamed in this studio."
      : "Modeled race: happy path adds 0 ms net — both start at t=0; the draft continues past the control-plane P50.",
  };
}

module.exports = { replayRuling, raceModel };
