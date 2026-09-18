const assert = require("assert");
const { replayRuling, raceModel } = require("./decide-route.cjs");

const base = {
  composite_risk: 10,
  composite_score: 80,
  questions: [
    { id: "jailbreak_attempt", top_probability: 0.03 },
    { id: "indirect_prompt_injection", top_probability: 0.02 },
    { id: "credential_or_pii_exposure", top_probability: 0.04 },
    { id: "citation_grounding", selected_choice: "verbatim_supported" },
    { id: "execution_tier", selected_choice: "tier1_fast_mini", confidence: 0.9 },
    { id: "selected_agent_skill", selected_choice: "none_needed" },
  ],
  field_verifications: [
    { field_name: "vendor", yes_prob: 0.97 },
    { field_name: "date", yes_prob: 0.09 },
  ],
};

const tau = {
  security_gate_confidence: 0.65,
  field_verify_confidence: 0.75,
  router_confidence: 0.8,
  composite_pass_threshold: 68,
};

assert.strictEqual(replayRuling(base, tau).final_route_tier, "TIER_2_SURGICAL_FIELD_REPAIR");
assert.strictEqual(
  replayRuling(
    { ...base, field_verifications: base.field_verifications.map((f) => ({ ...f, yes_prob: 0.96 })) },
    tau
  ).final_route_tier,
  "TIER_1_VERIFIED_FASTPATH"
);
const blockedReplay = replayRuling(
  {
    ...base,
    cascade: { aegis_control_cost_usd: 0.00004, naive_frontier_cost_usd: 0.0384 },
    questions: base.questions.map((q) =>
      q.id === "indirect_prompt_injection" ? { ...q, top_probability: 0.97 } : q
    ),
  },
  tau
);
assert.strictEqual(blockedReplay.final_route_tier, "TIER_0_BLOCK");
assert.ok(blockedReplay.final_decision.includes("Blocked before generation"));
assert.ok(blockedReplay.cascade.aegis_blended_cost_usd < 0.001);
assert.ok(blockedReplay.questions.find((q) => q.id === "indirect_prompt_injection").route_reason.includes("BLOCK"));

const locked = replayRuling(
  { ...base, field_verifications: base.field_verifications.map((f) => ({ ...f, yes_prob: 0.96 })) },
  tau
);
assert.ok(locked.final_decision.includes("All field nouls locked"));
assert.ok(locked.cascade.cost_savings_percent > 90);

const blocked = raceModel("TIER_0_BLOCK");
assert.strictEqual(blocked.abortAt, 114);
assert.ok(blocked.caption.includes("Modeled race"));
assert.strictEqual(raceModel("TIER_1_VERIFIED_FASTPATH").abortAt, null);

console.log("decide-route.cjs ok");
