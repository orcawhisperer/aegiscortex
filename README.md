# AegisCortex

**Aegis Speculative Control Plane · v1.0** — a loopback FinOps workbench, not a production gateway.

Aegis owns routing, field lock, surgical splice, and τ. The optional live scorer is TypeSafe Jev-1.13 behind a `SpeculativeScorer` interface (`typesafe-sdk-go` v0.6.0). The live scorer is not first-party; the studio names it once as `typesafe_jev`. Prefer `AEGIS_ENGINE_KEY`. `TYPESAFE_API_KEY` is a silent fallback.

The calibrated simulator inspects the JSON payload you send. Route labels, field rows, and flywheel counters are computed from those answers — not from the scenario name. Control-plane P50 is **114 ms**. Downstream Mini / Frontier spend is modeled unless a field is spliced.

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

Every evaluation binds **11 questions** (plus extras) in one `system_one` call:

| Stage | Questions | Effect |
| :--- | :--- | :--- |
| Security gate | `jailbreak_attempt`, `indirect_prompt_injection`, `credential_or_pii_exposure`, `policy_harm_severity` | Block before generation when a hazard noul ≥ τ_sec |
| Router | `execution_tier`, `selected_agent_skill` | Tier 0 / 1 / 2 with a confidence act-gate |
| Per-field gate | `verify_field_*`, `verify_rag_claim_grounding` | Lock fields independently; escalate only failures |
| Citation | `citation_grounding` | `verbatim_supported` vs `extrapolated` / `contradicted` |

Extra keys in `mini_model_extraction` / `extracted_fields` (beyond vendor / amount / date / claim) auto-synthesize `verify_field_*` nouls. Binding a JSON Schema replaces the default quartet.

The studio’s moat, on the sheet:

1. **Surgical repair slip** — frozen locked fields, micro-prompt, token budget vs 1,420-token full retry, red/black diff. Hallucinated notice date splices to **10/02/2026** (Nov 1 2026 − 30 days).
2. **0 ms τ replay** — sliders re-gate the last fan-out locally. **Run all 4** scores a portfolio.
3. **Click-to-highlight** evidence spans in the payload.
4. **SDK export** — copyable Go / TypeScript / curl with the current τ.
5. **Permalink** `?case=&sec=&field=&route=&comp=` and a print slip with SHA-256 of payload + `request_id`.
6. **Modeled speculative race** — honestly labeled; no live stream is cancelled.
7. **FinOps backtester** — `.jsonl` or 50-turn synthetic, monthly savings at 10M req/mo, confusion matrix.
8. **Schema compiler + extras** — bind any schema; extra extraction keys expand fan-out.

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
npm test
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

Or both via Vercel:

```bash
npx vercel dev
```

Live Jev (optional): set `AEGIS_ENGINE_KEY` on the backend process (`TYPESAFE_API_KEY` still works). Listen address is **not** hardcoded: Vercel sets `PORT` and Gin binds `:$PORT`. Locally, `AEGIS_ADDR` or `AEGIS_PORT` override the `127.0.0.1:8090` fallback. Next.js SSR uses `BACKEND_INTERNAL_URL` on Vercel (service binding) and `AEGIS_BACKEND_URL` locally.

## Deploy on Vercel

Same Services config as the official Next.js + Gin template.

1. Import the GitHub repo. Root `vercel.json` declares `frontend` (Next.js) and `backend` (`cmd/api/main.go`).
2. Optional: `AEGIS_ENGINE_KEY` in Project Settings → Environment Variables (Production and Preview). The browser key form is off on Vercel.
3. Deploy. Simulator works with no secrets.

Public routes:

- `/` — Next.js ledger
- `/api/hello` — Next.js route handler (starter-style ping)
- `/svc/api/status` — Gin ping
- `/svc/api/evaluate` — speculative fan-out
- `/svc/api/boot` — read-only first ruling (`?case=` honors the permalink)
- `/svc/api/backtest` — synthetic or `.jsonl` FinOps rollup

## Scenarios

1. **Indirect RAG injection** — hidden `SYSTEM OVERRIDE` → `TIER_0_BLOCK`
2. **Hallucinated contract date** — `09/15/2026` → `TIER_2_SURGICAL_FIELD_REPAIR` (splice `10/02/2026`)
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
| `POST` | `/svc/api/backtest` | `.jsonl` / synthetic turns → monthly savings + confusion |
| `POST` | `/svc/api/thresholds` | Slider gates (evaluations also send τ inline) |
| `POST` | `/svc/api/key` | In-memory engine key (loopback RemoteAddr only) |

## Production readiness

**Not production-ready as a gateway.** It is a demo workbench.

No operator auth, in-memory gates/flywheel/history, modeled (not executed) downstream spend, live evaluate rate-limited at 20/min per IP. Use it to demo and calibrate. Wire `Evaluate` behind your own gateway for real agents.
