const assert = require("assert");
const { localBackendOrigin, backendOrigin } = require("./backend-origin.cjs");

assert.strictEqual(localBackendOrigin({}), "http://127.0.0.1:8090");
assert.strictEqual(localBackendOrigin({ AEGIS_PORT: "9100" }), "http://127.0.0.1:9100");
assert.strictEqual(localBackendOrigin({ AEGIS_ADDR: "127.0.0.1:9200" }), "http://127.0.0.1:9200");
assert.strictEqual(
  localBackendOrigin({ AEGIS_BACKEND_URL: "http://127.0.0.1:9300/" }),
  "http://127.0.0.1:9300"
);
assert.strictEqual(
  backendOrigin({ BACKEND_INTERNAL_URL: "http://backend.internal" }),
  "http://backend.internal"
);
assert.strictEqual(backendOrigin({ VERCEL: "1" }), "");
assert.strictEqual(backendOrigin({}), "http://127.0.0.1:8090");

console.log("backend-origin.cjs ok");
