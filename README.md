# AegisCortex

**Aegis Speculative Control Plane · v1.0**

Local studio for an 11-question (plus extras) speculative AI control plane. Aegis owns routing, field lock, surgical splice, and τ. The optional live scorer is TypeSafe Jev-1.13 behind a `SpeculativeScorer` interface ([`typesafe-sdk-go` v0.6.0](https://github.com/orcawhisperer/typesafe-sdk-go)). The live scorer is **not** first-party; the studio names it once as `typesafe_jev`.

It is a **loopback FinOps workbench**, not a production gateway. The calibrated simulator inspects the JSON payload you send. Route labels, field rows, and flywheel counters come from those answers — not from the scenario name. Control-plane P50 is **114 ms**. Downstream Mini / Frontier spend is modeled unless a field is spliced.

Layout matches the [Next.js + Gin starter](https://vercel.com/templates/next.js/next-js-gin-starter): Next.js at `/`, Gin at `/svc/api`. The first ruling is rendered on the server (App Router, `force-dynamic`) via a private service binding; the browser then evaluates through `/svc/api`. Fonts are self-hosted with `next/font`.

```txt
aegiscortex/
├── backend/                 # Go 1.22 + Gin (`cmd/api/main.go`)
├── frontend/                # Next.js 15 App Router
├── vercel.json              # Vercel Services (frontend ↔ backend binding)
├── .env.example             # AEGIS_ENGINE_KEY + local bind
└── .github/workflows/ci.yml
```

## What it does

Every evaluation binds the shared security / router matrix plus per-field nouls in one `system_one` call:

| Stage | Questions | Effect |
| :--- | :--- | :--- |
| Security gate | `jailbreak_attempt`, `indirect_prompt_injection`, `credential_or_pii_exposure`, `policy_harm_severity` | Block before generation when a hazard noul ≥ τ_sec |
| Router | `execution_tier`, `selected_agent_skill` | Tier 0 / 1 / 2 with a confidence act-gate |
| Per-field gate | `verify_field_*`, `verify_rag_claim_grounding` | Lock fields independently; escalate only failures |
| Citation | `citation_grounding` | `verbatim_supported` vs `extrapolated` / `contradicted` |

Extra keys in `mini_model_extraction` / `extracted_fields` (beyond vendor / amount / date / claim) auto-synthesize `verify_field_*` nouls. Binding a JSON Schema or TypeScript interface replaces the default quartet.

On the sheet:

1. **Surgical repair slip** — frozen locked fields, micro-prompt, token budget vs a 1,420-token full retry, red/black diff. Hallucinated notice date splices to **10/02/2026** (anniversary of 1 Nov minus 30 days).
2. **0 ms τ replay** — sliders re-gate the last fan-out locally. **Run all 4** scores a portfolio.
3. **Click-to-highlight** evidence spans in the payload.
4. **SDK export** — copyable Go / TypeScript / curl with the current τ.
5. **Permalink** `?case=&sec=&field=&route=&comp=` and a print slip with SHA-256 of payload + `request_id`.
6. **Modeled speculative race** — honestly labeled; no live stream is cancelled.
7. **FinOps backtester** — `.jsonl` or 50-turn synthetic, monthly savings at 10M req/mo, confusion matrix.
8. **Schema compiler + extras** — bind any schema; extra extraction keys expand fan-out.

## Prerequisites

- Go 1.22+
- Node 22+ (CI uses 22; `npm ci` in `frontend/`)
- Optional: a TypeSafe key in `AEGIS_ENGINE_KEY` for live Jev

## Quickstart

Two processes. Gin listens on loopback; Next proxies `/svc/api` to it.

```bash
cd backend
go test ./...
go run ./cmd/api
```

```bash
cd frontend
npm ci
npm test
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

Or both via the Vercel CLI (same `vercel.json` Services layout):

```bash
npx vercel dev
```

## Environment

Copy [`.env.example`](.env.example). Never commit a real key.

| Variable | Where | Purpose |
| :--- | :--- | :--- |
| `AEGIS_ENGINE_KEY` | backend process / Vercel env | Preferred live-scorer key. Browser key form is off on Vercel. |
| `TYPESAFE_API_KEY` | backend process | Silent fallback when `AEGIS_ENGINE_KEY` is empty. |
| `PORT` | injected by Vercel | Gin binds `:$PORT` (all interfaces). Wins over local bind vars. |
| `AEGIS_ADDR` | local only | Loopback bind, e.g. `127.0.0.1:8090`. Rejected if not `127.0.0.1` / `localhost`. |
| `AEGIS_PORT` | local only | Port on `127.0.0.1` when `AEGIS_ADDR` is unset. |
| `AEGIS_BACKEND_URL` | Next.js SSR (local) | Origin for `GET /svc/api/boot`. Default `http://127.0.0.1:8090`. |
| `BACKEND_INTERNAL_URL` | Next.js on Vercel | Set by the service binding in `vercel.json`. Required on Vercel; no localhost fallback. |

`127.0.0.1:8090` is only the last-resort local default — not a baked-in production bind.

Precedence for Gin: **`PORT` → hosted-without-PORT `:3001` → `AEGIS_ADDR` → `AEGIS_PORT` → `127.0.0.1:8090`**.

Next.js rewrites `/svc/api/*` to `AEGIS_BACKEND_URL` / `AEGIS_ADDR` / `AEGIS_PORT` when `VERCEL` is unset. On Vercel the platform rewrite + `BACKEND_INTERNAL_URL` binding take over.

## Studio

`GET /` is `force-dynamic`. The server loads `GET /svc/api/boot?case=` so the first paint matches the permalink. Changing τ in the Gates sliders re-routes the last probabilities in the browser (0 ms). **Evaluate** is only needed when the payload or schema changes.

Permalink query:

```
/?case=sde_hallucinated_date&sec=0.65&field=0.75&route=0.80&comp=68
```

| Param | Gate |
| :--- | :--- |
| `case` | Preset id (`gw_rag_injection`, `sde_hallucinated_date`, `rag_verified_fastpath`, `triage_auto_refund`) |
| `sec` | `security_gate_confidence` |
| `field` | `field_verify_confidence` |
| `route` | `router_confidence` |
| `comp` | `composite_pass_threshold` |

Print ruling slip writes SHA-256(`request_id` + payload) into the colophon.

## Scenarios

| # | Id | Trigger | Route |
| :--- | :--- | :--- | :--- |
| 01 | `gw_rag_injection` | Hidden `SYSTEM OVERRIDE` | `TIER_0_BLOCK` |
| 02 | `sde_hallucinated_date` | `09/15/2026` notice date | `TIER_2_SURGICAL_FIELD_REPAIR` → splice `10/02/2026` |
| 03 | `rag_verified_fastpath` | 72-hour / €20M / 4% in source | `TIER_1_VERIFIED_FASTPATH` |
| 04 | `triage_auto_refund` | Two captured $49 charges | `TIER_0_AUTO_EXEC` |

## API

Gin public prefix is `/svc/api`. POSTs require `Content-Type: application/json` and reject `Sec-Fetch-Site: cross-site`. Live evaluate is 20/min per IP. `GET /boot` is read-only (does not mutate the flywheel or spend live Jev).

| Method | Path | Purpose |
| :--- | :--- | :--- |
| `GET` | `/svc/api/status` | Liveness (`ok`, `listen`, `hosted`) |
| `GET` | `/svc/api/healthz` | Same ping |
| `GET` | `/svc/api/state` | Presets, thresholds, flywheel |
| `GET` | `/svc/api/boot` | Read-only first ruling (`?case=` honors the permalink) |
| `POST` | `/svc/api/evaluate` | `{ "scenario_id", "context", "schema?", "schema_text?", "thresholds?", "read_only?" }` |
| `POST` | `/svc/api/compile` | JSON Schema / TypeScript → BindQuestions matrix |
| `POST` | `/svc/api/calibrate` | Pareto τ solver for a max escape rate |
| `POST` | `/svc/api/backtest` | `.jsonl` / `turns` / `synthetic` → monthly savings + confusion |
| `POST` | `/svc/api/thresholds` | Persist slider gates (evaluate also sends τ inline) |
| `POST` | `/svc/api/key` | In-memory engine key (**loopback `RemoteAddr` only**) |

Frontend also exposes `GET /api/hello` (starter-style ping).

### Evaluate

```bash
curl -sS -X POST http://127.0.0.1:8090/svc/api/evaluate \
  -H 'Content-Type: application/json' \
  -d '{
    "scenario_id": "sde_hallucinated_date",
    "thresholds": {
      "security_gate_confidence": 0.65,
      "field_verify_confidence": 0.75,
      "router_confidence": 0.8,
      "composite_pass_threshold": 68
    }
  }'
```

Response includes `final_route_tier`, `questions`, `field_verifications`, `surgical` (frozen fields + patches), `cascade`, `audit_hash`, `evidence`, and `scorer` (`aegis_calibrated_simulator` or `typesafe_jev`).

### Compile a schema

```bash
curl -sS -X POST http://127.0.0.1:8090/svc/api/compile \
  -H 'Content-Type: application/json' \
  -d '{
    "schema": {
      "title": "MSAExtraction",
      "type": "object",
      "properties": {
        "vendor_name": { "type": "string" },
        "notice_deadline_date": { "type": "string", "format": "date" }
      }
    }
  }'
```

`schema_text` also accepts a TypeScript `interface`. Send the compiled object (or `schema_text`) on the next `evaluate` to replace the default four field nouls.

### Backtest

Each `.jsonl` line is an `evaluate` body (`scenario_id` / `context` / optional `expected_tier`). Omit `jsonl` and set `synthetic` to replay the four presets.

```bash
curl -sS -X POST http://127.0.0.1:8090/svc/api/backtest \
  -H 'Content-Type: application/json' \
  -d '{"synthetic":50,"monthly_volume":10000000}'
```

Returns `tier_rates`, `monthly_aegis_usd`, `monthly_baseline_usd`, `savings_percent`, `confusion`, and `match_rate`.

### Calibrate τ

Needs labeled turns on the instance (run the four presets, or pass `expected_tier` on evaluate) before it can solve.

```bash
curl -sS -X POST http://127.0.0.1:8090/svc/api/calibrate \
  -H 'Content-Type: application/json' \
  -d '{"max_escape_rate":0.001}'
```

## Tests

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm test && npm run build
```

CI (`.github/workflows/ci.yml`) runs the same on every push to `main` and every pull request. Live Jev (`TestLiveTypeSafeFanout`) is skipped unless `TYPESAFE_API_KEY` is set in the job env.

## Deploy on Vercel

Same Services config as the official Next.js + Gin template.

1. Import the GitHub repo. Root `vercel.json` declares `frontend` (Next.js) and `backend` (`cmd/api/main.go`), binds `BACKEND_INTERNAL_URL`, and rewrites `/svc/api/*` to Gin.
2. Optional: `AEGIS_ENGINE_KEY` in Project Settings → Environment Variables (Production and Preview). The browser key form is off on Vercel.
3. Deploy. The simulator works with no secrets.

Public routes after deploy:

- `/` — Next.js ledger
- `/api/hello` — Next.js ping
- `/svc/api/status` — Gin ping
- `/svc/api/evaluate` — speculative fan-out
- `/svc/api/boot` — read-only first ruling
- `/svc/api/backtest` — FinOps rollup

## Production readiness

**Not production-ready as a gateway.** It is a demo workbench.

No operator auth. Gates, flywheel, and history are in-memory (per instance). Downstream Mini / Frontier cost is modeled, not executed. Live evaluate is rate-limited at 20/min per IP (socket peer off-Vercel; platform `X-Forwarded-For` only on Vercel). `/api/key` requires a loopback listen **and** a loopback `RemoteAddr` — spoofed `X-Forwarded-For` does not unlock it. `POST /backtest` clamps to 500 turns. CSRF on POST is content-type + `Sec-Fetch-Site` only.

**Solve τ** auto-seeds the four presets as read-only labeled turns on a cold instance. Slider replay recomputes the lede, cascade, and `route_reason` at 0 ms; click **Evaluate** to splice a newly failed field.

Use it to demo and calibrate. Wire `Evaluate` behind your own gateway for real agents.
