export default function TauBar({ value, tau, label }) {
  const pct = Math.max(0, Math.min(100, Math.round((Number(value) || 0) * 100)));
  const tauPct = Math.max(0, Math.min(100, Math.round((Number(tau) || 0) * 100)));
  const pass = (Number(value) || 0) >= (Number(tau) || 0);
  return (
    <span className={pass ? "tau-meter pass" : "tau-meter fail"} title={`${pct}% vs τ ${tauPct}%`}>
      <span className="tau-track" aria-hidden="true">
        <span className="tau-fill" style={{ width: `${pct}%` }} />
        <span className="tau-hair" style={{ left: `${tauPct}%` }} />
      </span>
      <span className="tau-readout">
        {pct}% <span className="tau-cut">τ {label ?? `${tauPct}%`}</span>
      </span>
    </span>
  );
}
