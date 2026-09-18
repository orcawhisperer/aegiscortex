# AegisCortex — 100ms Speculative AI Control Plane & Calibrated Arbitrage Engine

**Built on [`github.com/orcawhisperer/typesafe-sdk-go v0.6.0`](https://github.com/orcawhisperer/typesafe-sdk-go) & TypeSafe AI (`Jev-1.13`)**

AegisCortex is an inline **100ms Speculative AI Control Plane**, **3-Tier Economic Arbitrage Router**, **Per-Field Structured Data Verification Gate**, and **Self-Evolving Calibration Flywheel** with an embedded real-time FinOps & Probability Telemetry Studio (`127.0.0.1:8090`).

---

## Why AegisCortex? (When the Cost of Decision Approaches Zero)

Historically, every AI guardrail or LLM-as-a-Judge check cost **$0.15–$5.00/Mtok** and added **800ms–2,500ms** of serial latency. Teams were forced to ask at most *one* reactive question or route 100% of traffic to Frontier LLMs.

With TypeSafe AI (`Jev-1.13`):
- **`$0.042 / 1M input tokens`** (41.6× cheaper than GPT-4o-mini)
- **`$0.00` output token billing** (responses are calibrated probability distributions, not autoregressive token streams)
- **~114ms parallel speculative fan-out** (`0.4ms` GPU compute) across 10–50 orthogonal `Noul`, `Choice`, and `Score` questions sharing a single prefilled KV cache
- **RLCD Bayesian Calibration** (`0.961` AUROC across 35 benchmarks): when `Jev-1.13` reports `P = 0.87`, empirical accuracy is `87%`

AegisCortex exploits this paradigm shift to evaluate **11 simultaneous security, routing, triage, and per-field factual grounding questions** in **one ~114ms call (`$0.000034`)**:

| Stage | Primitive | Question IDs | Purpose |
| :--- | :--- | :--- | :--- |
| **Stage A: Inline Security Gate** | `3 × Noul` | `gw_prompt_injection`, `gw_pii_exfiltration`, `gw_policy_compliant` | Blocks indirect RAG prompt injection & PII leaks in 114ms before any LLM generation occurs. |
| **Stage B: Arbitrage Router & Triage** | `2 × Choice` + `2 × Score` | `route_complexity_tier`, `route_domain_specialist`, `route_reasoning_depth`, `triage_autonomous_action_safety` | Routes requests across **Tier 0** (Deterministic Code), **Tier 1** (Fast Mini LLM), or **Tier 2** (Frontier Reasoning). |
| **Stage C: Per-Field Surgical Verification** | `4 × Noul` (`BindQuestions`) | `verify_field_vendor_name`, `verify_field_invoice_amount`, `verify_field_invoice_date`, `verify_rag_claim_grounding` | Verifies each extracted JSON field independently. Locks verified fields (`P >= τ`) and escalates **only** the uncertain field. |

---

## Quickstart

```bash
# Run with Calibrated Jev-1.13 RLCD Simulation Engine (zero config)
go run .

# Or run against live TypeSafe AI API
TYPESAFE_API_KEY="ts_live_..." go run .
```

Then open **`http://127.0.0.1:8090`** in your browser.

---

## 4 Built-in Enterprise Scenarios

1. **Indirect RAG Prompt Injection Attack (`gw_rag_injection`)**:
   - Detects adversarial instructions embedded inside retrieved RAG chunks (`gw_prompt_injection = YES`, `P = 0.96`).
   - Short-circuits at **Tier 0 (`TIER_0_BLOCK`)** in `118ms` (**`99.8%` cost savings** vs Frontier + LLM Judge).
2. **Invoice SDE — Single-Field Date Hallucination (`sde_hallucinated_date`)**:
   - Verifies 4 extracted invoice fields in parallel via `typesafe.BindQuestions`.
   - Locks `vendor_name` (`P=0.98`), `invoice_amount` (`P=0.97`), and `rag_claim_grounding` (`P=0.93`), while catching that `invoice_date: 2026-02-15` is hallucinated (`P(NO)=0.89`).
   - Triggers **`TIER_2_SURGICAL_FIELD_REPAIR`** on *only* `invoice_date` (**`84.6%` cost savings**).
3. **Verified Enterprise Contract Fast-Path (`rag_verified_fastpath`)**:
   - All 4 extracted fields pass `τ = 0.90` (`P >= 0.94`).
   - Commits **`TIER_1_VERIFIED_FASTPATH`** immediately without calling a Frontier LLM (**`97.1%` cost savings**).
4. **Autonomous Billing Refund Triage (`triage_auto_refund`)**:
   - `route_complexity_tier` selects `tier0_deterministic_code` (`P=0.92`) and `triage_autonomous_action_safety` scores `5_safe_autonomous` (`P=0.89`).
   - Executes **`TIER_0_AUTO_EXEC`** webhook in `114ms` with `$0.00` LLM generation cost (**`99.8%` cost savings**).
