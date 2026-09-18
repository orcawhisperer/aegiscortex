"use client";

import { useCallback, useEffect, useMemo, useRef, useState, useTransition } from "react";

const BACKEND = process.env.NEXT_PUBLIC_BACKEND_URL || "/svc/api";

function tierClass(tier) {
  if (tier === "TIER_0_BLOCK") return "stamp tier-block";
  if (tier === "TIER_0_AUTO_EXEC") return "stamp tier-auto";
  if (tier && String(tier).indexOf("TIER_2") === 0) return "stamp tier-repair";
  return "stamp tier-fastpath";
}

function money(n) {
  return `$${(n || 0).toFixed(6)}`;
}

function Table({ headers, rows, rowClasses }) {
  return (
    <table>
      <thead>
        <tr>
          {headers.map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((cells, i) => (
          <tr key={i} className={rowClasses?.[i] || ""}>
            {cells.map((c, j) => (
              <td key={j}>{c}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function firstPreset(presets) {
  return (presets || []).find((p) => p.id === "rag_verified_fastpath") || (presets || [])[0];
}

export default function Studio({ boot }) {
  const initial = boot?.state;
  const first = firstPreset(initial?.presets);
  const [hosted, setHosted] = useState(Boolean(initial?.hosted));
  const [presets, setPresets] = useState(initial?.presets || []);
  const [activeId, setActiveId] = useState(first?.id || "rag_verified_fastpath");
  const [thresholds, setThresholds] = useState(
    initial?.thresholds || {
      security_gate_confidence: 0.65,
      field_verify_confidence: 0.75,
      router_confidence: 0.8,
      composite_pass_threshold: 68,
    }
  );
  const [contextJSON, setContextJSON] = useState(
    first ? JSON.stringify(first.context, null, 2) : "{}"
  );
  const [evalRes, setEvalRes] = useState(boot?.eval || null);
  const [status, setStatus] = useState(
    boot?.eval
      ? "Ready. Routing is computed from the payload, not from the case name."
      : boot?.error
        ? "Server could not reach Gin. Retrying in the browser…"
        : "Loading studio…"
  );
  const [statusError, setStatusError] = useState(Boolean(boot?.error && !boot?.eval));
  const [keyValue, setKeyValue] = useState("");
  const [flywheel, setFlywheel] = useState(boot?.eval?.flywheel || initial?.flywheel || null);
  const thresholdTimer = useRef(null);
  const [pending, startTransition] = useTransition();

  const modeLabel = useMemo(() => {
    if (evalRes?.fallback_used) return "Live failed — simulator";
    if (evalRes?.mode === "LIVE_TYPESAFE_API") return "Live API";
    return "Simulator";
  }, [evalRes]);

  const live = evalRes?.mode === "LIVE_TYPESAFE_API" && !evalRes?.fallback_used;

  const evaluate = useCallback(async (scenarioId, rawJSON) => {
    let parsed = {};
    try {
      parsed = rawJSON.trim() ? JSON.parse(rawJSON) : {};
    } catch (e) {
      setStatusError(true);
      setStatus("Invalid JSON: " + e.message);
      return;
    }
    setStatusError(false);
    setStatus("Running 11-question fan-out…");
    try {
      const r = await fetch(`${BACKEND}/evaluate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ scenario_id: scenarioId, context: parsed }),
        cache: "no-store",
      });
      const data = await r.json();
      if (!r.ok) {
        setStatusError(true);
        setStatus(data.error || "Evaluation failed");
        return;
      }
      startTransition(() => {
        setEvalRes(data);
        if (data.flywheel) setFlywheel(data.flywheel);
      });
      setStatus(
        `Route ${data.final_route_tier} in ${data.latency_ms.toFixed(0)} ms · ${data.questions.length} questions · savings ${data.cascade.cost_savings_percent.toFixed(1)}%`
      );
      if (data.live_error) {
        setStatusError(true);
        setStatus("Live API error, using simulator: " + data.live_error);
      }
    } catch (e) {
      setStatusError(true);
      setStatus("Evaluation failed: " + e.message);
    }
  }, []);

  useEffect(() => {
    if (boot?.state) return undefined;
    let cancelled = false;
    (async () => {
      try {
        const r = await fetch(`${BACKEND}/state`, { cache: "no-store" });
        const state = await r.json();
        if (cancelled) return;
        setHosted(Boolean(state.hosted));
        setPresets(state.presets || []);
        if (state.thresholds) setThresholds(state.thresholds);
        setFlywheel(state.flywheel || null);
        const preset = firstPreset(state.presets);
        if (preset) {
          setActiveId(preset.id);
          const raw = JSON.stringify(preset.context, null, 2);
          setContextJSON(raw);
          await evaluate(preset.id, raw);
        }
        setStatus("Ready. Routing is computed from the payload, not from the case name.");
        setStatusError(false);
      } catch (e) {
        if (!cancelled) {
          setStatusError(true);
          setStatus("Could not reach Gin backend at " + BACKEND + ": " + e.message);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [boot, evaluate]);

  async function persistThresholds(next) {
    setThresholds(next);
    await fetch(`${BACKEND}/thresholds`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(next),
    });
    await evaluate(activeId, contextJSON);
  }

  function selectCase(preset) {
    setActiveId(preset.id);
    const raw = JSON.stringify(preset.context, null, 2);
    setContextJSON(raw);
    evaluate(preset.id, raw);
  }

  async function holdKey(ev) {
    ev.preventDefault();
    const r = await fetch(`${BACKEND}/key`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ api_key: keyValue.trim() }),
    });
    const data = await r.json();
    setKeyValue("");
    if (!r.ok) {
      setStatusError(true);
      setStatus(data.error || "Key rejected");
      return;
    }
    setStatusError(false);
    setStatus(data.has_api_key ? "Live key stored in memory. Re-evaluating…" : "Key cleared. Simulator mode.");
    evaluate(activeId, contextJSON);
  }

  function applyRecommended() {
    if (!flywheel) return;
    persistThresholds({
      security_gate_confidence: flywheel.recommended_block_prob,
      field_verify_confidence: flywheel.recommended_verify_min,
      router_confidence: flywheel.recommended_act_gate,
      composite_pass_threshold: flywheel.recommended_composite,
    });
  }

  const cascade = evalRes?.cascade;
  const fields = evalRes?.field_verifications || [];
  const questions = evalRes?.questions || [];
  const history = evalRes?.history || [];

  return (
    <div className={pending ? "pending" : ""} aria-busy={pending}>
      <p className="folio">
        <span>{hosted ? "Vercel edition" : "Loopback edition"}</span>
        <span>TypeSafe Jev-1.13 · Next.js + Gin</span>
        <span>{hosted ? "Fluid Compute" : initial?.listen || "loopback"}</span>
      </p>

      <header className="masthead">
        <div className="masthead-left">
          <h1>AegisCortex</h1>
          <p className="standfirst">Eleven questions. One route. Costs modeled, not spent.</p>
        </div>
        <div className="masthead-right">
          <p className={live ? "mode live" : "mode sim"}>{modeLabel}</p>
          {hosted ? (
            <p className="note">
              Live Jev uses the server <code>TYPESAFE_API_KEY</code> env var.
            </p>
          ) : (
            <form className="key-form" onSubmit={holdKey} autoComplete="off">
              <label className="sr-only" htmlFor="api-key-input">
                TypeSafe API key
              </label>
              <input
                id="api-key-input"
                type="password"
                value={keyValue}
                onChange={(e) => setKeyValue(e.target.value)}
                placeholder="API key"
                spellCheck={false}
              />
              <button type="submit">Hold in memory</button>
            </form>
          )}
        </div>
      </header>

      <section className="ledger" aria-label="Run figures">
        <div>
          <span>Latency</span>
          <strong>{evalRes ? `${evalRes.latency_ms.toFixed(0)} ms` : "—"}</strong>
          <em>Jev P50 {cascade ? `${cascade.reference_jev_p50_ms.toFixed(0)} ms` : "114 ms"}</em>
        </div>
        <div>
          <span>Fan-out</span>
          <strong>{cascade ? money(cascade.aegis_control_cost_usd) : "—"}</strong>
          <em>$0.042 / 1M in</em>
        </div>
        <div>
          <span>Saved vs frontier</span>
          <strong>{cascade ? `${cascade.cost_savings_percent.toFixed(1)}%` : "—"}</strong>
          <em>baseline {cascade ? `$${cascade.naive_frontier_cost_usd.toFixed(5)}` : "—"}</em>
        </div>
        <div>
          <span>Trust</span>
          <strong>{evalRes ? evalRes.composite_score.toFixed(1) : "—"}</strong>
          <em>{evalRes?.final_route_tier || "—"}</em>
        </div>
        <div>
          <span>Runs</span>
          <strong>{flywheel ? String(flywheel.distilled_golden_examples) : "0"}</strong>
          <em>
            {flywheel ? `${flywheel.guardrails_blocked} block · ${flywheel.auto_executed} auto` : "—"}
          </em>
        </div>
      </section>

      <main className="sheet">
        <aside>
          <h2>Cases</h2>
          <div className="cases" role="radiogroup" aria-label="Preset cases">
            {presets.map((p, i) => (
              <button
                key={p.id}
                type="button"
                role="radio"
                aria-checked={p.id === activeId}
                className={p.id === activeId ? "scenario-btn active" : "scenario-btn"}
                onClick={() => selectCase(p)}
              >
                <div className="scenario-top">
                  <span className="scenario-title">
                    <span className="scenario-index">{String(i + 1).padStart(2, "0")}</span>
                    {p.title}
                  </span>
                  <span className="scenario-tag">{p.badge}</span>
                </div>
                <p className="scenario-desc">{p.description}</p>
              </button>
            ))}
          </div>

          <h2>Gates</h2>
          <button type="button" className="linkish" onClick={applyRecommended}>
            Use recommended τ
          </button>
          <div className="sliders">
            {[
              ["security_gate_confidence", "Security", 0.5, 0.99, 0.01, 2],
              ["field_verify_confidence", "Field", 0.5, 0.99, 0.01, 2],
              ["router_confidence", "Router", 0.5, 0.99, 0.01, 2],
              ["composite_pass_threshold", "Composite", 30, 95, 1, 0],
            ].map(([key, label, min, max, step, digits]) => (
              <label key={key}>
                {label} <b>{Number(thresholds[key]).toFixed(digits)}</b>
                <input
                  type="range"
                  min={min}
                  max={max}
                  step={step}
                  value={thresholds[key]}
                  onChange={(e) => {
                    const next = { ...thresholds, [key]: parseFloat(e.target.value) };
                    setThresholds(next);
                    clearTimeout(thresholdTimer.current);
                    thresholdTimer.current = setTimeout(() => persistThresholds(next), 220);
                  }}
                />
              </label>
            ))}
          </div>
          <p className="note">
            {flywheel
              ? `τ  sec=${flywheel.recommended_block_prob.toFixed(2)}  field=${flywheel.recommended_verify_min.toFixed(2)}  route=${flywheel.recommended_act_gate.toFixed(2)}  composite=${flywheel.recommended_composite.toFixed(0)}  ECE=${flywheel.expected_calibration_ece.toFixed(3)}`
              : ""}
          </p>

          <div className="run-row">
            <h2>Payload</h2>
            <button type="button" onClick={() => evaluate(activeId, contextJSON)}>
              Evaluate
            </button>
          </div>
          <label className="sr-only" htmlFor="context-editor">
            JSON context payload
          </label>
          <textarea
            id="context-editor"
            spellCheck={false}
            value={contextJSON}
            onChange={(e) => setContextJSON(e.target.value)}
            onKeyDown={(ev) => {
              if ((ev.metaKey || ev.ctrlKey) && ev.key === "Enter") {
                ev.preventDefault();
                evaluate(activeId, contextJSON);
              }
            }}
          />
          <p className={statusError ? "status error" : "status"} role="status">
            {status}
          </p>
        </aside>

        <section className="copy">
          <p className="kicker">Ruling</p>
          <div className="verdict-line">
            <h2>{evalRes?.final_route_tier || "—"}</h2>
            <p className={tierClass(evalRes?.final_route_tier)}>{evalRes?.final_route_tier || "—"}</p>
          </div>
          <p className="lede">{evalRes?.final_decision || ""}</p>
          <p className="note">
            {evalRes?.failed_fields?.length
              ? "Fields in repair: " + evalRes.failed_fields.join(", ")
              : evalRes?.selected_skill
                ? "Skill: " + evalRes.selected_skill
                : ""}
          </p>

          <table className="compare">
            <caption>Modeled spend — no downstream model is called</caption>
            <thead>
              <tr>
                <th></th>
                <th>Cost</th>
                <th>Time</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>Frontier + judge</td>
                <td>{cascade ? money(cascade.naive_frontier_cost_usd) : "—"}</td>
                <td>{cascade ? `${cascade.naive_frontier_latency_ms.toFixed(0)} ms` : "—"}</td>
              </tr>
              <tr>
                <td>Legacy router</td>
                <td>{cascade ? money(cascade.legacy_router_cost_usd) : "—"}</td>
                <td>{cascade ? `${cascade.legacy_router_latency_ms.toFixed(0)} ms` : "—"}</td>
              </tr>
              <tr>
                <td>This run</td>
                <td>{cascade ? money(cascade.aegis_blended_cost_usd) : "—"}</td>
                <td>{cascade ? `${cascade.aegis_total_latency_ms.toFixed(0)} ms` : "—"}</td>
              </tr>
            </tbody>
          </table>

          <h2>Fields</h2>
          <Table
            headers={["Field", "P(yes)", "Conf", "Gate"]}
            rows={fields.map((f) => [
              f.field_name,
              `${(f.yes_prob * 100).toFixed(0)}%`,
              f.confidence.toFixed(2),
              f.verified ? "lock" : "repair",
            ])}
            rowClasses={fields.map((f) => (f.verified ? "ok" : "fail"))}
          />

          <h2>Questions</h2>
          <Table
            headers={["Question", "Stage", "Answer", "P", "Gate"]}
            rows={questions.map((q) => [
              <div key={q.id}>
                <div>{q.id}</div>
                <div className="q-prompt">{q.prompt}</div>
              </div>,
              q.stage,
              q.selected_choice,
              `${(q.top_probability * 100).toFixed(0)}%`,
              String(q.route_reason || "").replace("Gate: ", ""),
            ])}
          />

          <h2>Log</h2>
          {history.length ? (
            <Table
              headers={["Time", "Route", "ms"]}
              rows={history.map((h) => [h.timestamp, h.final_route_tier, `${h.latency_ms.toFixed(0)} ms`])}
            />
          ) : (
            <p className="note">No runs yet.</p>
          )}
        </section>
      </main>

      <footer className="colophon">
        {hosted
          ? "Hosted workbench. Thresholds and the run log are per-instance and do not persist. Routes come from the payload, not the case name."
          : "Local workbench. The key never leaves this process. Routes come from the payload, not the case name."}
      </footer>
    </div>
  );
}
