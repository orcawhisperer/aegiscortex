export function highlightPayload(jsonText, needle) {
  if (!needle || !jsonText) return [{ text: jsonText, mark: false }];
  const parts = [];
  let remaining = jsonText;
  const raw = needle;
  while (remaining.length) {
    const idx = remaining.indexOf(raw);
    const lo = remaining.toLowerCase().indexOf(String(raw).toLowerCase());
    const at = idx >= 0 ? idx : lo;
    if (at < 0) {
      parts.push({ text: remaining, mark: false });
      break;
    }
    if (at > 0) parts.push({ text: remaining.slice(0, at), mark: false });
    parts.push({ text: remaining.slice(at, at + String(raw).length), mark: true });
    remaining = remaining.slice(at + String(raw).length);
  }
  return parts;
}

export function needleForRow(rowKey, evidence, contextJSON) {
  const hit = (evidence || []).find((e) => e.key === rowKey || e.key === rowKey?.replace("verify_field_", ""));
  if (hit?.needle) return hit.needle;
  if (!rowKey || !contextJSON) return "";
  if (rowKey.includes("injection") || rowKey.includes("jailbreak")) {
    if (contextJSON.includes("SYSTEM OVERRIDE")) return "SYSTEM OVERRIDE";
  }
  if (rowKey.includes("date") || rowKey.includes("notice")) {
    const m = contextJSON.match(/0?9\/15\/2026/);
    if (m) return m[0];
  }
  return "";
}
