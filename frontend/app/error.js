"use client";

export default function Error({ error, reset }) {
  return (
    <main className="copy">
      <p className="kicker">Fault</p>
      <div className="verdict-line">
        <h2>The sheet did not set</h2>
      </div>
      <p className="lede">{error?.message || "Unknown render error."}</p>
      <p className="note">The Next.js frontend failed before the ledger could paint.</p>
      <p>
        <button type="button" onClick={() => reset()}>
          Try again
        </button>
      </p>
    </main>
  );
}
