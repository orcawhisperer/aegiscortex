export default function Primer() {
  return (
    <details className="primer">
      <summary>New here? 60-second control-plane primer</summary>
      <div className="primer-body">
        <p>
          <strong>Why this exists.</strong> Status quo is a slow frontier model plus a judge
          (~11.4 s, $0.0384) — or a mini model that hallucinates one date and forces a full-document
          retry. Aegis fires one 114 ms fan-out (~$0.00004): block attacks at $0, auto-fire a safe
          tool at $0, or freeze the good JSON and splice only the broken field (tens of tokens, not
          1,420).
        </p>
        <p>
          <strong>Why 11 questions in one call?</strong> Shared KV prefill. Eleven verification
          heads share one forward pass. That is the control-plane P50 of 114 ms — not eleven
          separate judge calls.
        </p>
        <dl>
          <div>
            <dt>Noul</dt>
            <dd>Calibrated 0–100% that a binary fact or hazard is true. Shown as P(yes).</dd>
          </div>
          <div>
            <dt>Choice</dt>
            <dd>Calibrated distribution over mutually exclusive routes or skills.</dd>
          </div>
          <div>
            <dt>Score</dt>
            <dd>Calibrated 0.0–2.0 harm band (Negligible / Moderate / Severe).</dd>
          </div>
          <div>
            <dt>τ (tau)</dt>
            <dd>
              Your cutoff. τ_sec blocks, τ_field locks a field, τ_route allows auto-exec. Drag a
              slider: the hairline moves across every bar.
            </dd>
          </div>
          <div>
            <dt>Escape rate</dt>
            <dd>Share of hallucinations allowed through unflagged. Solve τ to meet an SLA.</dd>
          </div>
        </dl>
        <p className="primer-story">
          The four cases are the four exhaustive outcomes: <b>block</b> an attack → <b>repair</b> one
          field → <b>approve</b> a verified draft → <b>auto-execute</b> a refund.
        </p>
      </div>
    </details>
  );
}
