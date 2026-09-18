export default function Loading() {
  return (
    <>
      <p className="folio">
        <span>Setting the sheet</span>
        <span>TypeSafe Jev-1.13 · Next.js + Gin</span>
        <span>SSR</span>
      </p>
      <header className="masthead">
        <div className="masthead-left">
          <h1>AegisCortex</h1>
          <p className="standfirst">Reading the first case from Gin…</p>
        </div>
      </header>
      <section className="ledger" aria-hidden="true">
        {["Latency", "Fan-out", "Saved vs frontier", "Trust", "Runs"].map((label) => (
          <div key={label}>
            <span>{label}</span>
            <strong className="sk sk-lg" />
            <em className="sk" />
          </div>
        ))}
      </section>
      <main className="sheet">
        <aside>
          <h2>Cases</h2>
          <div className="sk sk-block" />
          <div className="sk sk-block" />
          <div className="sk sk-block" />
        </aside>
        <section className="copy">
          <p className="kicker">Ruling</p>
          <div className="sk sk-lg" />
          <p className="sk sk-block" />
        </section>
      </main>
    </>
  );
}
