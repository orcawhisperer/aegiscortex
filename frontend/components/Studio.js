"use client";

import { useCallback, useEffect, useMemo, useRef, useState, useTransition } from "react";
import { replayRuling, raceModel } from "../lib/decide-route.cjs";
import { exportSnippets } from "../lib/export-sdk";
import { highlightPayload, needleForRow } from "../lib/highlight";
import { writeStudioURL, sha256Hex } from "../lib/permalink";

const BACKEND = process.env.NEXT_PUBLIC_BACKEND_URL || "/svc/api";

const DEFAULT_SCHEMA = `{
  "title": "MSAExtraction",
  "type": "object",
  "properties": {
    "vendor_name": { "type": "string" },
    "contract_value_usd": { "type": "number" },
    "effective_date": { "type": "string", "format": "date" },
    "notice_deadline_date": { "type": "string", "format": "date" },
    "auto_renews": { "type": "boolean" }
  }
}`;

function tierClass(tier) {
  if (tier === "TIER_0_BLOCK") return "stamp tier-block";
  if (tier === "TIER_0_AUTO_EXEC") return "stamp tier-auto";
  if (tier && String(tier).indexOf("TIER_2") === 0) return "stamp tier-repair";
  return "stamp tier-fastpath";
}

function money(n) {
  return `$${(n || 0).toFixed(6)}`;
}

function Table({ caption, headers, rows, rowClasses, onRowClick }) {
  return (
    <table>
      {caption ? <caption>{caption}</caption> : null}
      <thead>
        <tr>
          {headers.map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((cells, i) => (
          <tr
            key={i}
            className={rowClasses?.[i] || ""}
            onClick={onRowClick ? () => onRowClick(i) : undefined}
            style={onRowClick ? { cursor: "pointer" } : undefined}
          >
            {cells.map((c, j) => (
              <td key={j}>{c}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function firstPreset(presets, preferred) {
  return (
    (presets || []).find((p) => p.id === preferred) ||
    (presets || []).find((p) => p.id === "rag_verified_fastpath") ||
    (presets || [])[0]
  );
}

export default function Studio({ boot, permalink }) {
  const initial = boot?.state;
  const first = firstPreset(initial?.presets, permalink?.case);
  const [hosted, setHosted] = useState(Boolean(initial?.hosted));
  const [presets, setPresets] = useState(initial?.presets || []);
  const [activeId, setActiveId] = useState(first?.id || "rag_verified_fastpath");
  const [thresholds, setThresholds] = useState(permalink?.thresholds || initial?.thresholds || {
    security_gate_confidence: 0.65,
    field_verify_confidence: 0.75,
    router_confidence: 0.8,
    composite_pass_threshold: 68,
  });
  const [contextJSON, setContextJSON] = useState(first ? JSON.stringify(first.context, null, 2) : "{}");
  const [rawEval, setRawEval] = useState(boot?.eval || null);
  const [status, setStatus] = useState(
    boot?.eval ? "Ready. τ replay is local. Evaluate only when the payload changes." : boot?.error ? "Server could not reach Gin. Retrying…" : "Loading studio…"
  );
  const [statusError, setStatusError] = useState(Boolean(boot?.error && !boot?.eval));
  const [keyValue, setKeyValue] = useState("");
  const [flywheel, setFlywheel] = useState(boot?.eval?.flywheel || initial?.flywheel || null);
  const [schemaText, setSchemaText] = useState(DEFAULT_SCHEMA);
  const [compiled, setCompiled] = useState(null);
  const [useSchema, setUseSchema] = useState(false);
  const [escapeRate, setEscapeRate] = useState(0.001);
  const [highlight, setHighlight] = useState("");
  const [portfolio, setPortfolio] = useState(null);
  const [sdkLang, setSdkLang] = useState("go");
  const [backtest, setBacktest] = useState(null);
  const [printHash, setPrintHash] = useState("");
  const evalAbort = useRef(null);
  const [pending, startTransition] = useTransition();

  const evalRes = useMemo(() => replayRuling(rawEval, thresholds) || rawEval, [rawEval, thresholds]);
  const modeLabel = useMemo(() => {
    if (rawEval?.fallback_used) return "Live failed — simulator";
    if (rawEval?.mode === "LIVE_TYPESAFE_API") return "Live engine";
    return "Simulator";
  }, [rawEval]);
  const live = rawEval?.mode === "LIVE_TYPESAFE_API" && !rawEval?.fallback_used;

  const evaluate = useCallback(async (scenarioId, rawJSON, extra = {}) => {
    let parsed = {};
    try {
      parsed = rawJSON.trim() ? JSON.parse(rawJSON) : {};
    } catch (e) {
      setStatusError(true);
      setStatus("Invalid JSON: " + e.message);
      return null;
    }
    if (evalAbort.current) evalAbort.current.abort();
    const controller = new AbortController();
    evalAbort.current = controller;
    setStatusError(false);
    setStatus("Running speculative fan-out…");
    try {
      const body = {
        scenario_id: scenarioId,
        context: parsed,
        thresholds,
        read_only: Boolean(extra.read_only),
      };
      if (useSchema) {
        try {
          body.schema = JSON.parse(schemaText);
        } catch {
          body.schema_text = schemaText;
        }
      }
      const r = await fetch(`${BACKEND}/evaluate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        cache: "no-store",
        signal: controller.signal,
      });
      const data = await r.json();
      if (!r.ok) {
        setStatusError(true);
        setStatus(data.error || "Evaluation failed");
        return null;
      }
      if (!extra.silent) {
        startTransition(() => {
          setRawEval(data);
          if (data.flywheel) setFlywheel(data.flywheel);
        });
        setStatus(`Route ${data.final_route_tier} in ${data.latency_ms.toFixed(0)} ms · ${data.question_count || data.questions.length} heads`);
        if (data.live_error) {
          setStatusError(true);
          setStatus("Live engine error, using simulator: " + data.live_error);
        }
      }
      return data;
    } catch (e) {
      if (e.name === "AbortError") return null;
      setStatusError(true);
      setStatus("Evaluation failed: " + e.message);
      return null;
    }
  }, [schemaText, thresholds, useSchema]);

  useEffect(() => {
    writeStudioURL(activeId, thresholds);
  }, [activeId, thresholds]);

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
        setFlywheel(state.flywheel || null);
        const preset = firstPreset(state.presets, permalink?.case);
        if (preset) {
          setActiveId(preset.id);
          const raw = JSON.stringify(preset.context, null, 2);
          setContextJSON(raw);
          await evaluate(preset.id, raw);
        }
        setStatus("Ready. τ replay is local.");
        setStatusError(false);
      } catch (e) {
        if (!cancelled) {
          setStatusError(true);
          setStatus("Could not reach control plane at " + BACKEND + ": " + e.message);
        }
      }
    })();
    return () => {
      cancelled = true;
      if (evalAbort.current) evalAbort.current.abort();
    };
  }, [boot]); // eslint-disable-line react-hooks/exhaustive-deps

  function selectCase(preset) {
    setActiveId(preset.id);
    const raw = JSON.stringify(preset.context, null, 2);
    setContextJSON(raw);
    setHighlight("");
    evaluate(preset.id, raw);
  }

  function onCaseKey(ev, index) {
    if (ev.key !== "ArrowDown" && ev.key !== "ArrowUp") return;
    ev.preventDefault();
    const next = ev.key === "ArrowDown" ? Math.min(presets.length - 1, index + 1) : Math.max(0, index - 1);
    const preset = presets[next];
    if (preset) {
      selectCase(preset);
      ev.currentTarget.parentElement?.querySelectorAll("[role=radio]")[next]?.focus();
    }
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
    setStatus(data.has_api_key ? "Engine key held in memory." : "Key cleared. Simulator.");
    evaluate(activeId, contextJSON);
  }

  function applyRecommended() {
    if (!flywheel) return;
    setThresholds({
      security_gate_confidence: flywheel.recommended_block_prob,
      field_verify_confidence: flywheel.recommended_verify_min,
      router_confidence: flywheel.recommended_act_gate,
      composite_pass_threshold: flywheel.recommended_composite,
    });
  }

  async function compileSchema() {
    try {
      const r = await fetch(`${BACKEND}/compile`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ schema_text: schemaText }),
      });
      const data = await r.json();
      if (!r.ok) {
        setStatusError(true);
        setStatus(data.error || "Schema compile failed");
        return;
      }
      setCompiled(data);
      setUseSchema(true);
      setStatus(`Bound ${data.fields?.length || 0} field nouls.`);
    } catch (e) {
      setStatusError(true);
      setStatus("Schema compile failed: " + e.message);
    }
  }

  async function solveTau() {
    try {
      const r = await fetch(`${BACKEND}/calibrate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ max_escape_rate: escapeRate }),
      });
      const data = await r.json();
      if (!r.ok) {
        setStatusError(true);
        setStatus(data.error || "Need labeled runs before solving τ");
        return;
      }
      setThresholds(data.thresholds);
      setStatus(`Solved τ · match ${(data.match_rate * 100).toFixed(0)}% on ${data.samples} labels.`);
    } catch (e) {
      setStatusError(true);
      setStatus("Calibrate failed: " + e.message);
    }
  }

  async function runPortfolio() {
    const out = {};
    for (const p of presets) {
      const data = await evaluate(p.id, JSON.stringify(p.context, null, 2), { read_only: true, silent: true });
      if (data) out[p.id] = data;
    }
    setPortfolio(out);
    setStatus("Portfolio scored. Drag τ to re-gate all four cases at 0 ms.");
  }

  async function runBacktest(jsonl, synthetic) {
    try {
      const r = await fetch(`${BACKEND}/backtest`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          jsonl: jsonl || "",
          synthetic: synthetic || 0,
          thresholds,
          monthly_volume: 10000000,
        }),
      });
      const data = await r.json();
      if (!r.ok) {
        setStatusError(true);
        setStatus(data.error || "Backtest failed");
        return;
      }
      setBacktest(data);
      setStatus(`Backtest ${data.turns} turns · ${data.savings_percent.toFixed(1)}% vs unrouted frontier.`);
    } catch (e) {
      setStatusError(true);
      setStatus("Backtest failed: " + e.message);
    }
  }

  async function printSlip() {
    const text = `${evalRes?.request_id || ""}\n${contextJSON}`;
    const hex = evalRes?.audit_hash || (await sha256Hex(text));
    setPrintHash(hex);
    setTimeout(() => window.print(), 50);
  }

  const cascade = evalRes?.cascade;
  const fields = evalRes?.field_verifications || [];
  const questions = evalRes?.questions || [];
  const history = evalRes?.history || [];
  const surgical = evalRes?.failed_fields?.length ? rawEval?.surgical : null;
  const snippets = exportSnippets({ thresholds, questions, pipeline: evalRes?.pipeline });
  const race = raceModel(evalRes?.final_route_tier);
  const marked = highlight ? highlightPayload(contextJSON, highlight) : [{ text: contextJSON, mark: false }];

  return (
    <div className={pending ? "pending" : ""} aria-busy={pending}>
      <p className="folio">
        <span>{hosted ? "Vercel edition" : "Loopback edition"}</span>
        <span>Aegis Speculative Control Plane · v1.0</span>
        <span>{hosted ? "Fluid Compute" : initial?.listen || "loopback"}</span>
      </p>

      <header className="masthead">
        <div className="masthead-left">
          <h1>AegisCortex</h1>
          <p className="standfirst">Bind any schema. Repair one field. Calibrate τ.</p>
        </div>
        <div className="masthead-right">
          <p className={live ? "mode live" : "mode sim"} aria-live="polite">{modeLabel}</p>
          {hosted ? (
            <p className="note">Live scorer uses <code>AEGIS_ENGINE_KEY</code> (or legacy <code>TYPESAFE_API_KEY</code>).</p>
          ) : (
            <form className="key-form" onSubmit={holdKey} autoComplete="off">
              <label className="sr-only" htmlFor="api-key-input">Engine key</label>
              <input id="api-key-input" type="password" value={keyValue} onChange={(e) => setKeyValue(e.target.value)} placeholder="AEGIS_ENGINE_KEY" spellCheck={false} />
              <button type="submit">Hold in memory</button>
            </form>
          )}
        </div>
      </header>

      <section className="ledger" aria-label="Run figures">
        <div>
          <span>Latency</span>
          <strong>{evalRes ? `${evalRes.latency_ms.toFixed(0)} ms` : <span className="sk-lg" />}</strong>
          <em>Control plane P50 {cascade ? `${cascade.reference_jev_p50_ms.toFixed(0)} ms` : "114 ms"}</em>
        </div>
        <div>
          <span>Fan-out</span>
          <strong>{cascade ? money(cascade.aegis_control_cost_usd) : <span className="sk-lg" />}</strong>
          <em>{evalRes ? `${evalRes.question_count || questions.length} heads` : "—"}</em>
        </div>
        <div>
          <span>Saved vs frontier</span>
          <strong>{cascade ? `${cascade.cost_savings_percent.toFixed(1)}%` : <span className="sk-lg" />}</strong>
          <em>baseline {cascade ? `$${cascade.naive_frontier_cost_usd.toFixed(5)}` : "—"}</em>
        </div>
        <div>
          <span>Trust</span>
          <strong>{evalRes ? evalRes.composite_score.toFixed(1) : <span className="sk-lg" />}</strong>
          <em>{evalRes?.final_route_tier || "—"}</em>
        </div>
        <div>
          <span>Runs</span>
          <strong>{flywheel ? String(flywheel.distilled_golden_examples) : "0"}</strong>
          <em>{flywheel ? `${flywheel.guardrails_blocked} block · ${flywheel.auto_executed} auto` : "—"}</em>
        </div>
      </section>

      <main className="sheet">
        <aside>
          <h2>Cases</h2>
          <div className="cases" role="radiogroup" aria-label="Preset cases">
            {presets.map((p, i) => (
              <button key={p.id} type="button" role="radio" aria-checked={p.id === activeId} tabIndex={p.id === activeId ? 0 : -1} className={p.id === activeId ? "scenario-btn active" : "scenario-btn"} onClick={() => selectCase(p)} onKeyDown={(ev) => onCaseKey(ev, i)}>
                <div className="scenario-top">
                  <span className="scenario-title"><span className="scenario-index">{String(i + 1).padStart(2, "0")}</span>{p.title}</span>
                  <span className="scenario-tag">{p.badge}</span>
                </div>
                <p className="scenario-desc">{p.description}</p>
              </button>
            ))}
          </div>

          <h2>Gates</h2>
          <div className="moat-actions">
            <button type="button" className="linkish" onClick={applyRecommended}>Use recommended τ</button>
            <button type="button" className="linkish" onClick={printSlip}>Print ruling slip</button>
          </div>
          <div className="sliders">
            {[
              ["security_gate_confidence", "Security", 0.5, 0.99, 0.01, 2],
              ["field_verify_confidence", "Field", 0.5, 0.99, 0.01, 2],
              ["router_confidence", "Router", 0.5, 0.99, 0.01, 2],
              ["composite_pass_threshold", "Composite", 30, 95, 1, 0],
            ].map(([key, label, min, max, step, digits]) => (
              <label key={key}>
                {label} <b>{Number(thresholds[key]).toFixed(digits)}</b>
                <input type="range" min={min} max={max} step={step} value={thresholds[key]} onInput={(e) => setThresholds({ ...thresholds, [key]: parseFloat(e.target.value) })} onChange={(e) => setThresholds({ ...thresholds, [key]: parseFloat(e.target.value) })} />
              </label>
            ))}
            <label>
              Max escape <b>{(escapeRate * 100).toFixed(2)}%</b>
              <input type="range" min={0.001} max={0.05} step={0.001} value={escapeRate} onChange={(e) => setEscapeRate(parseFloat(e.target.value))} />
            </label>
          </div>
          <div className="moat-actions">
            <button type="button" onClick={solveTau}>Solve τ</button>
            <button type="button" onClick={runPortfolio}>Run all 4</button>
          </div>

          <h2>Schema</h2>
          <p className="note">New keys in <code>extracted_fields</code> / <code>mini_model_extraction</code> become field nouls automatically. Bind a full schema to replace the default four.</p>
          <textarea id="schema-editor" className="schema-box" spellCheck={false} value={schemaText} onChange={(e) => setSchemaText(e.target.value)} />
          <div className="moat-actions">
            <button type="button" onClick={compileSchema}>Bind schema</button>
            <button type="button" className="linkish" onClick={() => { setUseSchema(false); setCompiled(null); }}>Default + extras</button>
          </div>

          <div className="run-row">
            <h2>Payload</h2>
            <button type="button" onClick={() => evaluate(activeId, contextJSON)}>Evaluate</button>
          </div>
          {highlight ? (
            <pre className="payload-mark" aria-label="Highlighted payload">{marked.map((p, i) => p.mark ? <mark key={i}>{p.text}</mark> : <span key={i}>{p.text}</span>)}</pre>
          ) : null}
          <textarea id="context-editor" spellCheck={false} value={contextJSON} onChange={(e) => setContextJSON(e.target.value)} onKeyDown={(ev) => { if ((ev.metaKey || ev.ctrlKey) && ev.key === "Enter") { ev.preventDefault(); evaluate(activeId, contextJSON); } }} />
          <p className={statusError ? "status error" : "status"} role="status">{status}</p>

          <h2>FinOps batch</h2>
          <div className="moat-actions">
            <button type="button" onClick={() => runBacktest("", 50)}>50-turn synthetic</button>
            <label className="linkish">
              Upload .jsonl
              <input type="file" accept=".jsonl,application/jsonl,text/plain" className="sr-only" onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                runBacktest(await file.text(), 0);
              }} />
            </label>
          </div>
        </aside>

        <section className="copy">
          <p className="kicker">Ruling</p>
          <div className="verdict-line">
            <h2>{evalRes?.final_route_tier || "—"}</h2>
            <p className={tierClass(evalRes?.final_route_tier)}>{evalRes?.final_route_tier || "—"}</p>
          </div>
          <p className="lede">{evalRes?.final_decision || ""}</p>
          {evalRes?.replayed ? <p className="note">Counterfactual re-gate at 0 ms — probabilities unchanged.</p> : null}

          <div className="race" aria-label="Modeled speculative race">
            <div className="race-track">
              <span className="race-jev" style={{ width: "12%" }} />
              {race.abortAt ? <span className="race-abort" /> : <span className="race-llm" />}
            </div>
            <p className="note">{race.caption}</p>
            <p className="note">0 ms ──► 114 ms [control plane] {race.abortAt ? "──X── 2,400 ms [stream cancelled]" : "────── 2,400 ms [draft continues]"}</p>
          </div>

          {surgical?.executed ? (
            <div className="patch slip">
              <h2>Surgical repair slip</h2>
              <p className="note">Micro-prompt {surgical.token_budget} tokens vs {surgical.full_retry_tokens || 1420} for a full-document retry.</p>
              <p className="lede">{surgical.micro_prompt}</p>
              <Table caption="Frozen fields — locked, not regenerated" headers={["Field", "P(yes)"]} rows={(surgical.frozen_fields || []).map((f) => [f.field, `${((f.yes_prob || 0) * 100).toFixed(0)}%`])} rowClasses={(surgical.frozen_fields || []).map(() => "ok")} />
              <div className="diff">
                {(surgical.patches || []).map((p) => (
                  <p key={p.field}>
                    <span className="del">- {JSON.stringify(p.field)}: {JSON.stringify(p.before)}</span>
                    <br />
                    <span className="add">+ {JSON.stringify(p.field)}: {JSON.stringify(p.after)}</span>
                  </p>
                ))}
              </div>
            </div>
          ) : null}

          {portfolio ? (
            <div className="patch">
              <h2>Portfolio</h2>
              <Table
                caption="Four presets re-gated with the current τ"
                headers={["Case", "Tier"]}
                rows={presets.map((p) => {
                  const replayed = replayRuling(portfolio[p.id], thresholds);
                  return [p.title, replayed?.final_route_tier || "—"];
                })}
              />
            </div>
          ) : null}

          {backtest ? (
            <div className="patch">
              <h2>Executive FinOps ledger</h2>
              <Table
                caption={`Projected at ${(backtest.monthly_volume / 1e6).toFixed(0)}M req/mo`}
                headers={["Tier", "Share", "Turns"]}
                rows={Object.entries(backtest.tier_rates || {}).map(([k, v]) => [k, `${(v * 100).toFixed(1)}%`, String(backtest.tier_counts[k] || 0)])}
              />
              <p className="lede">
                ${backtest.monthly_baseline_usd.toFixed(0)}/mo unrouted → ${backtest.monthly_aegis_usd.toFixed(0)}/mo Aegis ({backtest.savings_percent.toFixed(1)}% net).
              </p>
              <p className="note">Confusion {JSON.stringify(backtest.confusion)} · match {(backtest.match_rate * 100).toFixed(0)}% on {backtest.labeled} labeled turns.</p>
            </div>
          ) : null}

          <table className="compare">
            <caption>Control-plane spend is metered. Downstream Mini/Frontier is modeled unless a field is spliced.</caption>
            <thead><tr><th></th><th>Cost</th><th>Time</th></tr></thead>
            <tbody>
              <tr><td>Frontier + judge</td><td>{cascade ? money(cascade.naive_frontier_cost_usd) : "—"}</td><td>{cascade ? `${cascade.naive_frontier_latency_ms.toFixed(0)} ms` : "—"}</td></tr>
              <tr><td>This run</td><td>{cascade ? money(cascade.aegis_blended_cost_usd) : "—"}</td><td>{cascade ? `${cascade.aegis_total_latency_ms.toFixed(0)} ms` : "—"}</td></tr>
            </tbody>
          </table>

          <h2>Fields</h2>
          <Table
            caption="Click a row to ink the trigger in the payload"
            headers={["Field", "P(yes)", "Conf", "Gate"]}
            rows={fields.map((f) => [f.field_name, `${(f.yes_prob * 100).toFixed(0)}%`, f.confidence.toFixed(2), f.verified ? "lock" : "repair"])}
            rowClasses={fields.map((f) => (f.verified ? "ok" : "fail"))}
            onRowClick={(i) => setHighlight(needleForRow(fields[i].field_name, rawEval?.evidence, contextJSON) || fields[i].field_name)}
          />

          <h2>Questions</h2>
          <Table
            caption="Click a row to underline the evidence span"
            headers={["Question", "Stage", "Answer", "P", "Gate"]}
            rows={questions.map((q) => [
              <div key={q.id}><div>{q.id}</div><div className="q-prompt">{q.prompt}</div></div>,
              q.stage,
              q.selected_choice,
              `${(q.top_probability * 100).toFixed(0)}%`,
              String(q.route_reason || "").replace("Gate: ", ""),
            ])}
            onRowClick={(i) => setHighlight(needleForRow(questions[i].id, rawEval?.evidence, contextJSON) || questions[i].id)}
          />

          <h2>Code / SDK</h2>
          <div className="moat-actions">
            {["go", "ts", "curl"].map((lang) => (
              <button key={lang} type="button" className={sdkLang === lang ? "" : "linkish"} onClick={() => setSdkLang(lang)}>{lang}</button>
            ))}
            <button type="button" className="linkish" onClick={() => navigator.clipboard.writeText(snippets[sdkLang])}>Copy</button>
          </div>
          <pre className="sdk">{snippets[sdkLang]}</pre>

          <h2>Log</h2>
          {history.length ? (
            <Table caption="Recent labeled turns" headers={["Time", "Route", "ms"]} rows={history.map((h) => [h.timestamp, h.final_route_tier, `${h.latency_ms.toFixed(0)} ms`])} />
          ) : (
            <p className="note">No mutating runs yet. Page load is read-only.</p>
          )}
        </section>
      </main>

      <footer className="colophon">
        {hosted
          ? "Hosted workbench. τ is inline. Flywheel is per-instance. Live evaluate is rate-limited."
          : "Local workbench. Engine key hold is loopback-only."}
        {printHash || evalRes?.audit_hash ? ` SHA-256 ${printHash || evalRes.audit_hash}` : ""}
        {compiled?.stub && useSchema ? "" : ""}
      </footer>
    </div>
  );
}
