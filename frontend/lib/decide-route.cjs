const MODELED_MINI = 0.00085;
const MODELED_FRONTIER = 0.0285;
const MODELED_UNROUTED = 0.0384;

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
  const fieldByQuestion = Object.fromEntries(fields.map((f) => [f.question_id, f]));

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
  let final_decision = `All field nouls locked at τ_field=${tauField.toFixed(2)} and citation is ${citation || "verbatim_supported"} (trust=${trust.toFixed(1)}/100 ≥ ${tauComp.toFixed(0)}). Serve the mini draft; skip frontier reasoning.`;
  if (guardrail || risk >= tauSec) {
    final_route_tier = "TIER_0_BLOCK";
    final_decision = `Blocked before generation. Guardrail noul or composite risk (${risk.toFixed(2)}) met τ_sec=${tauSec.toFixed(2)}. Modeled downstream LLM cost is $0.00.`;
  } else if (failed.length || citation === "contradicted" || citation === "extrapolated") {
    final_route_tier = "TIER_2_SURGICAL_FIELD_REPAIR";
    const fieldNote = failed.length ? `failed fields: ${failed.join(", ")}` : `citation=${citation}`;
    final_decision = `Per-field verifier refused to lock every extraction (${fieldNote}). Escalating only the uncertain work to frontier reasoning. Trust=${trust.toFixed(1)}/100.`;
  } else if (trust < tauComp) {
    final_route_tier = "TIER_2_SURGICAL_FIELD_REPAIR";
    final_decision = `Composite quality (${trust.toFixed(1)}/100) is below the pass gate (${tauComp.toFixed(0)}). Routing to Tier-2 review.`;
  } else if (
    exec.selected_choice === "tier0_deterministic" &&
    (exec.confidence || 0) >= tauRoute &&
    skill !== "none_needed" &&
    skill !== "incident_pager_alert"
  ) {
    final_route_tier = "TIER_0_AUTO_EXEC";
    final_decision = `High-confidence policy match (execution_tier conf=${Number(exec.confidence || 0).toFixed(2)} ≥ τ_route=${tauRoute.toFixed(2)}). Modeled action: \`${skill}\` with $0.00 generation cost. No webhook is fired from this studio.`;
  }

  const control = Number(evalRes.cascade?.aegis_control_cost_usd || 0.000041);
  let blended = control + MODELED_MINI;
  if (final_route_tier === "TIER_0_BLOCK" || final_route_tier === "TIER_0_AUTO_EXEC") {
    blended = control;
  } else if (final_route_tier === "TIER_2_SURGICAL_FIELD_REPAIR") {
    blended = failed.length ? control + MODELED_MINI + MODELED_FRONTIER : control + MODELED_FRONTIER;
  }
  const baseline = Number(evalRes.cascade?.naive_frontier_cost_usd || MODELED_UNROUTED);
  const cascade = {
    ...(evalRes.cascade || {}),
    aegis_blended_cost_usd: blended,
    cost_savings_percent: baseline > 0 ? ((baseline - blended) / baseline) * 100 : 0,
  };

  const replayedQuestions = questions.map((row) => {
    let reason = row.route_reason;
    if (row.id === "jailbreak_attempt" || row.id === "indirect_prompt_injection" || row.id === "credential_or_pii_exposure") {
      reason = (row.top_probability || 0) >= tauSec ? "Gate: BLOCK" : "Gate: PASS";
    } else if (row.id === "execution_tier") {
      reason =
        final_route_tier === "TIER_0_AUTO_EXEC" && (row.confidence || 0) >= tauRoute ? "Gate: ACT" : "Gate: PASS";
    } else if (String(row.id || "").startsWith("verify_field_") || row.id === "verify_rag_claim_grounding") {
      const fv = fieldByQuestion[row.id] || fields.find((f) => f.field_name === row.id);
      reason = fv && !fv.verified ? "Gate: ESCALATE" : "Gate: PASS";
    }
    return { ...row, route_reason: reason };
  });

  return {
    ...evalRes,
    final_route_tier,
    final_decision,
    failed_fields: failed,
    field_verifications: fields,
    questions: replayedQuestions,
    cascade,
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
