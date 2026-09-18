function firstParam(sp, key) {
  if (!sp) return "";
  if (typeof sp.get === "function") return sp.get(key) || "";
  const v = sp[key];
  if (Array.isArray(v)) return v[0] || "";
  return v == null ? "" : String(v);
}

function numParam(sp, key, fallback) {
  const v = parseFloat(firstParam(sp, key));
  return Number.isFinite(v) ? v : fallback;
}

export function parseStudioParams(sp) {
  const caseId = firstParam(sp, "case");
  const hasTau =
    firstParam(sp, "sec") !== "" ||
    firstParam(sp, "field") !== "" ||
    firstParam(sp, "route") !== "" ||
    firstParam(sp, "comp") !== "";
  return {
    case: caseId,
    thresholds:
      caseId || hasTau
        ? {
            security_gate_confidence: numParam(sp, "sec", 0.65),
            field_verify_confidence: numParam(sp, "field", 0.75),
            router_confidence: numParam(sp, "route", 0.8),
            composite_pass_threshold: numParam(sp, "comp", 68),
          }
        : null,
  };
}

export function readStudioURL() {
  if (typeof window === "undefined") return { case: "", thresholds: null };
  return parseStudioParams(new URLSearchParams(window.location.search));
}

export function writeStudioURL(caseId, thresholds) {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (caseId) url.searchParams.set("case", caseId);
  url.searchParams.set("sec", Number(thresholds.security_gate_confidence).toFixed(2));
  url.searchParams.set("field", Number(thresholds.field_verify_confidence).toFixed(2));
  url.searchParams.set("route", Number(thresholds.router_confidence).toFixed(2));
  url.searchParams.set("comp", Number(thresholds.composite_pass_threshold).toFixed(0));
  window.history.replaceState({}, "", url);
}

export async function sha256Hex(text) {
  const buf = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
