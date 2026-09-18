export const CASE_WATCH = {
  gw_rag_injection: {
    outcome: "01 · Block the attack",
    ribbon:
      "Passage #2 hides a SYSTEM OVERRIDE. Watch the security head spike (≥ τ_sec) and abort at 114 ms before spending $0.00 on generation. Try: click indirect_prompt_injection to ink the attack, delete that line, then Evaluate.",
  },
  sde_hallucinated_date: {
    outcome: "02 · Repair one field",
    ribbon:
      "The mini model turned “30 days prior to Nov 1” into 09/15/2026. Watch three fields lock while only the notice date (low P(yes)) is spliced to 10/02/2026 — not a 1,420-token redo.",
  },
  rag_verified_fastpath: {
    outcome: "03 · Approve the fast draft",
    ribbon:
      "All four extracted facts match the GDPR source (≥ τ_field). Aegis serves the cheap mini draft and skips the frontier judge. Try: drag Field above 0.95 and watch the stamp escalate at 0 ms.",
  },
  triage_auto_refund: {
    outcome: "04 · Auto-execute the refund",
    ribbon:
      "Two captured $49 charges match the duplicate-charge policy. Aegis fires billing_refund_exec with $0.00 generation cost. No webhook is sent from this studio.",
  },
};

export const ROUTE_TO_CASE = {
  TIER_0_BLOCK: "gw_rag_injection",
  TIER_2_SURGICAL_FIELD_REPAIR: "sde_hallucinated_date",
  TIER_1_VERIFIED_FASTPATH: "rag_verified_fastpath",
  TIER_0_AUTO_EXEC: "triage_auto_refund",
};

export function watchFor(caseId) {
  return (
    CASE_WATCH[caseId] || {
      outcome: "Custom payload",
      ribbon: "Eleven cheap questions decide the route from this JSON. Drag τ to re-gate at 0 ms. Evaluate only when the payload changes.",
    }
  );
}
