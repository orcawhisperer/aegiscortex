export function highlightPayload(jsonText, needle) {
  if (!needle || !jsonText) return [{ text: jsonText, mark: false }];
  const parts = [];
  let remaining = jsonText;
  const raw = String(needle);
  while (remaining.length) {
    const idx = remaining.indexOf(raw);
    const lo = remaining.toLowerCase().indexOf(raw.toLowerCase());
    const at = idx >= 0 ? idx : lo;
    if (at < 0) {
      parts.push({ text: remaining, mark: false });
      break;
    }
    if (at > 0) parts.push({ text: remaining.slice(0, at), mark: false });
    parts.push({ text: remaining.slice(at, at + raw.length), mark: true });
    remaining = remaining.slice(at + raw.length);
  }
  return parts;
}

function evidenceHit(rowKey, evidence) {
  const key = String(rowKey || "");
  const bare = key.replace(/^verify_field_/, "").replace(/^verify_rag_/, "");
  return (evidence || []).find((e) => {
    const ek = String(e.key || "");
    if (!ek) return false;
    if (ek === key || ek === bare) return true;
    if (ek.includes(bare) || bare.includes(ek)) return true;
    if ((key.includes("date") || key.includes("notice")) && (ek.includes("date") || ek.includes("notice") || ek.includes("deadline"))) {
      return true;
    }
    if (key.includes("vendor") && ek.includes("vendor")) return true;
    if (key.includes("injection") && ek.includes("injection")) return true;
    return false;
  });
}

export function needleForRow(rowKey, evidence, contextJSON) {
  const hit = evidenceHit(rowKey, evidence);
  if (hit?.needle) return hit.needle;
  if (!rowKey || !contextJSON) return "";
  const key = String(rowKey);
  if (key.includes("injection") || key.includes("jailbreak")) {
    if (contextJSON.includes("SYSTEM OVERRIDE")) return "SYSTEM OVERRIDE";
  }
  if (key.includes("date") || key.includes("notice") || key.includes("deadline")) {
    const m = contextJSON.match(/0?9\/15\/2026/);
    if (m) return m[0];
  }
  try {
    const obj = JSON.parse(contextJSON);
    const ext = obj.mini_model_extraction || obj.extracted_fields || {};
    const tokens = key
      .replace(/^verify_field_/, "")
      .replace(/^verify_rag_/, "")
      .split(/[_-]+/)
      .filter((t) => t.length > 2);
    let notice = "";
    let other = "";
    for (const [k, v] of Object.entries(ext)) {
      const kl = k.toLowerCase();
      const s = v == null ? "" : String(v);
      if (!s || !contextJSON.includes(s)) continue;
      const related = tokens.some((t) => kl.includes(t.toLowerCase()));
      if (!related) continue;
      if (kl.includes("notice") || kl.includes("deadline")) notice = s;
      else if (!other) other = s;
    }
    if ((key.includes("notice") || key.includes("deadline") || key.includes("date")) && notice) {
      return notice;
    }
    if (notice || other) return notice || other;
    if (key.includes("vendor")) {
      const m = contextJSON.match(/[A-Z][A-Za-z0-9&., ]+(?:LLC|Inc|Ltd)/);
      if (m) return m[0];
    }
  } catch {
    /* keep the payload unmarked */
  }
  return "";
}
