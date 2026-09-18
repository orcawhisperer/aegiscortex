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

If the live call fails, the UI says so and falls back to the simulator. Locally the key stays in process memory.

Local bind must be loopback (`AEGIS_ADDR` defaults to `127.0.0.1:8090`). On Vercel the server listens on `PORT`.

## Deploy on Vercel

This is a **Go Framework Preset** app: root `go.mod` + `main.go`. `vercel.json` sets `"framework": "go"`.

1. Import the GitHub repo in Vercel (Framework Preset: **Go**) or run `npx vercel` from the repo root.
2. Optional: add `TYPESAFE_API_KEY` under Project Settings → Environment Variables (Production and Preview). The browser “Hold in memory” form is disabled on Vercel so visitors cannot plant a key on a shared instance.
3. Deploy. Simulator mode works with no env vars.

This hosted studio is a **demo**, not a production control plane. See “Production readiness” below.

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
| `POST` | `/api/key` | In-memory TypeSafe key (local only; 403 on Vercel) |

## Production readiness

**Not production-ready as a gateway.** It is a loopback / hosted workbench.

| Ready | Not ready |
| :--- | :--- |
| Honest routing from the payload | No operator auth on the public studio |
| Self-contained Go binary / Vercel Go preset | In-memory thresholds, flywheel, and history (lost on cold start; racy across isolates) |
| CSP / XFO / loopback-only local bind | Does not execute refunds or call Mini/Frontier |
| Live Jev with env-var key | No durable store, no rate limits, no multi-tenant isolation |
| Tests on payload content | Flywheel ECE is an in-process estimate, not a published benchmark |

Use it to demo and calibrate. To run in front of real agents, extract `CortexEngine.Evaluate` behind auth, persist gates, and execute routes in your own gateway.

## Notes

- Simulator wall-clock is typically a few milliseconds. The KPI also shows the published Jev P50 (~114ms) as a **reference**, not a padded measurement.
- `CompositePassThreshold` is a real gate (trust × 100).
- Flywheel ECE is an in-process running estimate from observed nouls; it is not a published benchmark reprint.
