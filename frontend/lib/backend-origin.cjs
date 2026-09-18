function stripSlash(value) {
  return String(value || "").replace(/\/$/, "");
}

function asOrigin(addr) {
  const trimmed = String(addr || "").trim();
  if (!trimmed) return "";
  if (/^https?:\/\//i.test(trimmed)) return stripSlash(trimmed);
  return `http://${trimmed}`;
}

/** Local Gin origin. Never uses Next's PORT (that is the frontend). */
function localBackendOrigin(env = process.env) {
  if (env.AEGIS_BACKEND_URL) return stripSlash(env.AEGIS_BACKEND_URL);
  if (env.AEGIS_ADDR) return asOrigin(env.AEGIS_ADDR);
  return `http://127.0.0.1:${env.AEGIS_PORT || "8090"}`;
}

/** SSR origin: Vercel service binding first; no localhost fallback on Vercel. */
function backendOrigin(env = process.env) {
  if (env.BACKEND_INTERNAL_URL) return stripSlash(env.BACKEND_INTERNAL_URL);
  if (env.AEGIS_BACKEND_URL) return stripSlash(env.AEGIS_BACKEND_URL);
  if (env.VERCEL) return "";
  return localBackendOrigin(env);
}

module.exports = { localBackendOrigin, backendOrigin };
