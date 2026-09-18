# AegisCortex

Local studio for an **11-question speculative AI control plane** built on [`typesafe-sdk-go` v0.6.0](https://github.com/orcawhisperer/typesafe-sdk-go) and TypeSafe AI (`Jev-1.13`).

It is a **loopback FinOps workbench**, not a production gateway. The calibrated simulator inspects the JSON payload you send. Route labels, field rows, and flywheel counters are computed from those answers — not from the scenario name. The studio is a paper ledger, not a glow-card dashboard.

## What it does

Every evaluation binds **11 questions** in one `system_one` call:

| Stage | Questions | Effect |
| :--- | :--- | :--- |
| Security gate | `jailbreak_attempt`, `indirect_prompt_injection`, `credential_or_pii_exposure`, `policy_harm_severity` | Block before generation when a hazard noul ≥ τ_sec |
| Router | `execution_tier`, `selected_agent_skill` | Tier 0 / 1 / 2 with a confidence act-gate |
| Per-field gate | `verify_field_vendor_name`, `verify_field_invoice_amount`, `verify_field_invoice_date`, `verify_rag_claim_grounding` | Lock fields independently; escalate only failures |
| Citation | `citation_grounding` | `verbatim_supported` vs `extrapolated` / `contradicted` |

Downstream Mini / Frontier costs in the ledger are **modeled** (“if this route were executed”). The studio does not call those LLMs and does not fire refund webhooks.

## Quickstart

```bash
go test ./...
go run .
```

Open [http://127.0.0.1:8090](http://127.0.0.1:8090).

Live Jev (optional):

```bash
TYPESAFE_API_KEY="ts_live_..." go run .
```

If the live call fails, the UI says so and falls back to the simulator. The key stays in process memory.

Bind address must be loopback (`AEGIS_ADDR` defaults to `127.0.0.1:8090`).

## Scenarios

1. **Indirect RAG injection** — hidden `SYSTEM OVERRIDE` in a retrieved passage → `TIER_0_BLOCK`
2. **Hallucinated contract date** — relative “30 days prior” extracted as `09/15/2026` → `TIER_2_SURGICAL_FIELD_REPAIR` on that field
3. **Verified GDPR RAG** — 72-hour / €20M / 4% all in source → `TIER_1_VERIFIED_FASTPATH`
4. **Duplicate-charge refund** — two captured $49 charges + explicit policy → `TIER_0_AUTO_EXEC` (`billing_refund_exec`)

Edit the JSON and re-run. Changing the payload changes the route.

## API

| Method | Path | Purpose |
| :--- | :--- | :--- |
| `GET` | `/` | Studio |
| `GET` | `/healthz` | Liveness |
| `GET` | `/api/state` | Presets, thresholds, flywheel |
| `POST` | `/api/evaluate` | `{ "scenario_id", "context" }` |
| `POST` | `/api/thresholds` | Slider gates, including composite score |
| `POST` | `/api/key` | In-memory TypeSafe key |

## Notes

- Simulator wall-clock is typically a few milliseconds. The KPI also shows the published Jev P50 (~114ms) as a **reference**, not a padded measurement.
- `CompositePassThreshold` is a real gate (trust × 100).
- Flywheel ECE is an in-process running estimate from observed nouls; it is not a published benchmark reprint.
