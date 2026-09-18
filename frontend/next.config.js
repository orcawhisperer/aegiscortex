const path = require("path");
const { localBackendOrigin } = require("./lib/backend-origin.cjs");

/** @type {import("next").NextConfig} */
const nextConfig = {
  turbopack: {
    root: path.join(__dirname),
  },
  async rewrites() {
    if (process.env.VERCEL) return [];
    const origin = localBackendOrigin();
    return [
      {
        source: "/svc/api/:path*",
        destination: `${origin}/svc/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
