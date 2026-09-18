import { anatomyModel } from "../lib/anatomy";
import { ROUTE_TO_CASE } from "../lib/watch-copy";

const ROUTES = [
  { id: "TIER_0_BLOCK", label: "Tier 0 Block", cost: "$0" },
  { id: "TIER_0_AUTO_EXEC", label: "Tier 0 Auto-exec", cost: "$0" },
  { id: "TIER_1_VERIFIED_FASTPATH", label: "Tier 1 Fast path", cost: "mini" },
  { id: "TIER_2_SURGICAL_FIELD_REPAIR", label: "Tier 2 Surgical repair", cost: "splice" },
];

function pct(n) {
  return `${((n || 0) * 100).toFixed(0)}%`;
}

export default function Anatomy({ evalRes, thresholds, onPickRoute }) {
  const a = anatomyModel(evalRes, thresholds);
  return (
    <details className="anatomy" open>
      <summary>Anatomy of a 114 ms ruling</summary>
      <div className="anatomy-flow" aria-label="Control-plane stages">
        <div className="anatomy-col">
          <p className="anatomy-k">1 · JSON state + draft</p>
          <p>Left-rail JSON. The mini extraction is a hypothesis, not a verdict.</p>
        </div>
        <div className="anatomy-col anatomy-fan">
          <p className="anatomy-k">
            2 · Single-pass {a.heads}-head fan-out
          </p>
          <p>
            {a.latency.toFixed(0)} ms · ${a.cost.toFixed(5)}
          </p>
          <ul>
            <li>
              {a.security.count} security heads · risk {pct(a.security.risk)}{" "}
              {a.security.pass ? "<" : "≥"} τ_sec {pct(a.security.tau)} {a.security.pass ? "✓" : "✗"}
            </li>
            <li>
              {a.router.count} router heads · conf {pct(a.router.conf)}{" "}
              {a.router.pass ? "≥" : "<"} τ_rte {pct(a.router.tau)}
            </li>
            <li>
              {a.fields.count} field / cite heads · {a.fields.locked} locked ✓ · {a.fields.failed}{" "}
              failed ✗ · τ_field {pct(a.fields.tau)}
            </li>
          </ul>
        </div>
        <div className="anatomy-col">
          <p className="anatomy-k">3 · Active route</p>
          <div className="anatomy-routes" role="group" aria-label="Load a case by route">
            {ROUTES.map((r) => (
              <button
                key={r.id}
                type="button"
                className={a.route === r.id ? "anatomy-route active" : "anatomy-route"}
                onClick={() => onPickRoute?.(ROUTE_TO_CASE[r.id])}
              >
                <span>{a.route === r.id ? "●" : "○"}</span> {r.label}
                <em>{r.cost}</em>
              </button>
            ))}
          </div>
        </div>
      </div>
    </details>
  );
}
