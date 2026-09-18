# AegisCortex

Local studio for an **11-question speculative AI control plane** built on [`typesafe-sdk-go` v0.6.0](https://github.com/orcawhisperer/typesafe-sdk-go) and TypeSafe AI (`Jev-1.13`).

It is a **loopback FinOps workbench**, not a production gateway. The calibrated simulator inspects the JSON payload you send. Route labels, field rows, and flywheel counters are computed from those answers — not from the scenario name.

The studio’s moat is the control plane: bind an arbitrary JSON Schema into per-field `DefineNoul` questions on one `SystemOne` prefill, splice only the failed fields (surgical repair), and solve τ from labeled turns against a hallucination-escape SLA.

Layout matches the [Next.js + Gin starter](https://vercel.com/templates/next.js/next-js-gin-starter): Next.js at `/`, Gin at `/svc/api`. The first ruling is rendered on the server (App Router, `force-dynamic`) via a private service binding; the browser then evaluates through `/svc/api`. Fonts are self-hosted with `next/font`.

```txt
aegiscortex/
├── backend/                 # Go + Gin
│   ├── cmd/api/main.go
│   └── …
├── frontend/                # Next.js App Router
│   └── app/
└── vercel.json              # Vercel Services
```

## What it does

Every evaluation binds **11 questions** in one `system_one` call:

| Stage | Questions | Effect |
| :--- | :--- | :--- |
| Security gate | `jailbreak_attempt`, `indirect_prompt_injection`, `credential_or_pii_exposure`, `policy_harm_severity` | Block before generation when a hazard noul ≥ τ_sec |
| Router | `execution_tier`, `selected_agent_skill` | Tier 0 / 1 / 2 with a confidence act-gate |
| Per-field gate | `verify_field_vendor_name`, `verify_field_invoice_amount`, `verify_field_invoice_date`, `verify_rag_claim_grounding` | Lock fields independently; escalate only failures |
| Citation | `citation_grounding` | `verbatim_supported` vs `extrapolated` / `contradicted` |

Downstream Mini / Frontier costs in the ledger are **modeled**. The studio does not call those LLMs and does not fire refund webhooks.

## Quickstart

Backend (loopback):

```bash
cd backend
go test ./...
go run ./cmd/api
```

Frontend (proxies `/svc/api` to `AEGIS_BACKEND_URL` / `AEGIS_ADDR` / `AEGIS_PORT`, default `127.0.0.1:8090`, when not on Vercel):

```bash
cd frontend
npm ci
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

Or both via Vercel:

```bash
npx vercel dev
```

Live Jev (optional): set `TYPESAFE_API_KEY` on the backend process. Listen address is **not** hardcoded: Vercel sets `PORT` and Gin binds `:$PORT`. Locally, `AEGIS_ADDR` or `AEGIS_PORT` override the `127.0.0.1:8090` fallback. Next.js SSR uses `BACKEND_INTERNAL_URL` on Vercel (service binding) and `AEGIS_BACKEND_URL` locally.

## Deploy on Vercel

Same Services config as the official Next.js + Gin template.

1. Import the GitHub repo. Root `vercel.json` declares `frontend` (Next.js) and `backend` (`cmd/api/main.go`).
2. Optional: `TYPESAFE_API_KEY` in Project Settings → Environment Variables (Production and Preview). The browser key form is off on Vercel.
3. Deploy. Simulator works with no secrets.

Public routes:

- `/` — Next.js ledger
- `/api/hello` — Next.js route handler (starter-style ping)
- `/svc/api/status` — Gin ping
- `/svc/api/evaluate` — 11-question fan-out

## Scenarios

1. **Indirect RAG injection** — hidden `SYSTEM OVERRIDE` → `TIER_0_BLOCK`
2. **Hallucinated contract date** — `09/15/2026` → `TIER_2_SURGICAL_FIELD_REPAIR`
3. **Verified GDPR RAG** — 72-hour / €20M / 4% in source → `TIER_1_VERIFIED_FASTPATH`
4. **Duplicate-charge refund** — two captured $49 charges → `TIER_0_AUTO_EXEC`

## API (Gin, `/svc/api`)

| Method | Path | Purpose |
| :--- | :--- | :--- |
| `GET` | `/svc/api/status` | Liveness |
| `GET` | `/svc/api/state` | Presets, thresholds, flywheel |
| `GET` | `/svc/api/boot` | Read-only first ruling (does not mutate flywheel) |
| `POST` | `/svc/api/evaluate` | `{ "scenario_id", "context", "schema?", "thresholds?" }` |
| `POST` | `/svc/api/compile` | JSON Schema / TypeScript → BindQuestions matrix |
| `POST` | `/svc/api/calibrate` | Pareto τ solver for a max escape rate |
| `POST` | `/svc/api/thresholds` | Slider gates (evaluations also send τ inline) |
| `POST` | `/svc/api/key` | In-memory TypeSafe key (loopback RemoteAddr only) |

## Production readiness

**Not production-ready as a gateway.** It is a demo workbench.

No operator auth, in-memory gates/flywheel/history, modeled (not executed) downstream spend, no rate limits or tenant isolation. Use it to demo and calibrate. Wire `Evaluate` behind your own gateway for real agents.
