const LOCAL_BACKEND = "http://127.0.0.1:8090";

export function backendOrigin() {
  return (
    process.env.BACKEND_INTERNAL_URL ||
    process.env.AEGIS_BACKEND_URL ||
    LOCAL_BACKEND
  );
}

export function backendURL(path) {
  const suffix = path.startsWith("/svc/api")
    ? path
    : `/svc/api${path.startsWith("/") ? path : `/${path}`}`;
  return new URL(suffix, backendOrigin()).toString();
}

export async function fetchBackend(path, init = {}) {
  const res = await fetch(backendURL(path), {
    ...init,
    cache: "no-store",
    headers: {
      Accept: "application/json",
      ...(init.headers || {}),
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Backend ${res.status} for ${path}`);
  }
  return data;
}

export async function loadStudioBoot() {
  try {
    const state = await fetchBackend("/state");
    const first =
      (state.presets || []).find((p) => p.id === "rag_verified_fastpath") ||
      (state.presets || [])[0];
    if (!first) {
      return { state, eval: null, error: null };
    }
    const evaluation = await fetchBackend("/evaluate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ scenario_id: first.id, context: first.context }),
    });
    return { state, eval: evaluation, error: null };
  } catch (err) {
    return { state: null, eval: null, error: err.message };
  }
}
