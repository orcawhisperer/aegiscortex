import { backendOrigin as resolveBackendOrigin } from "./backend-origin.cjs";

export function backendOrigin() {
  const origin = resolveBackendOrigin();
  if (!origin) {
    throw new Error("BACKEND_INTERNAL_URL is required on Vercel (service binding)");
  }
  return origin;
}

export function backendURL(path) {
  const suffix = path.startsWith("/svc/api")
    ? path
    : `/svc/api${path.startsWith("/") ? path : `/${path}`}`;
  return new URL(suffix, backendOrigin()).toString();
}

export async function fetchBackend(path, init = {}) {
  const method = (init.method || "GET").toUpperCase();
  const timeoutMs = init.timeoutMs || (method === "POST" ? 25000 : 8000);
  const { timeoutMs: _ignored, ...rest } = init;
  const res = await fetch(backendURL(path), {
    ...rest,
    cache: "no-store",
    signal: init.signal || AbortSignal.timeout(timeoutMs),
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(init.headers || {}),
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Backend ${res.status} for ${path}`);
  }
  return data;
}

export async function loadStudioBoot(caseId) {
  try {
    const q = caseId ? `?case=${encodeURIComponent(caseId)}` : "";
    const boot = await fetchBackend(`/boot${q}`);
    return { state: boot.state || null, eval: boot.eval || null, error: null };
  } catch (err) {
    return { state: null, eval: null, error: err.message };
  }
}
